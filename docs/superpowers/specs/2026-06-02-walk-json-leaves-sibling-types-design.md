# Same Message Type 多路径 Query 遍历修复设计文档

**日期**: 2026-06-02  
**项目**: protoc-gen-typescript-http  
**状态**: **已实施**（已修复并验证，未 commit 进 git）  
**关联**: 评审 `2026-06-02-golang-tool-project-review.md` P0 #3

---

## 1. 问题描述

`internal/plugin/jsonleafwalk.go` 的 `jsonWalker` 用 message `FullName` 作为全局 `seen` key。当同一 message 类型在 request 中不同字段路径下第二次出现时，`enter()` 返回 `false`，整棵子树被静默丢弃。

```
.proto
  message Address { string city = 1; string street = 2; }
  message RouteRequest { Address source = 1; Address destination = 2; }
  rpc Search(RouteRequest) returns (Route) {
    option (google.api.http) = {
      get: "/v1/routes"
    };
  }
```

**当前（错误）行为**：
- 遍历到 `source`（Address 类型）→ `enter(Address)` → true，加入 seen
- 遍历到 `destination`（也是 Address 类型）→ `enter(Address)` → false（已在 seen 中）
- 整棵 `destination.*` 子树被静默丢弃
- 生成的 query 只包含 `source.city`、`source.street`，**`destination.city`、`destination.street` 全部漏掉**

影响：静默缺参，调用方不带任何错误信息，排查成本极高。

---

## 2. 根因分析

| 层面 | 实际情况 |
|---|---|
| 原意 | 防止自引用 message 类型（如 `TreeNode { TreeNode child }`）无限递归 |
| 实际实现 | `seen map[FullName]struct{}` 是整次遍历的全局 set，不是当前路径栈 |
| 副作用 | 同一类型在兄弟节点第二次出现时也被阻止下钻 |

`seen` 的语义被错位实现：「祖先链中已有该类型」（防递归）被实现成「整次遍历已访问过该类型」（防重访）。

---

## 3. 修复方案

将 `jsonWalker.seen` 改为 `jsonWalker.ancestors`（路径栈），配对 `enter` / `leave`：

| 维度 | 原 | 修复后 |
|---|---|---|
| 数据结构 | `map[FullName]struct{}` | 同上 |
| 生命周期 | 整次遍历累计，进入后永不删除 | 当前路径栈，`defer leave(name)` 退出时删除 |
| 含义 | "已访问过" | "在当前祖先链中" |

**关键代码改动**（`internal/plugin/jsonleafwalk.go`）：

```go
type jsonWalker struct {
    ancestors map[protoreflect.FullName]struct{}
}

func (w *jsonWalker) enter(name protoreflect.FullName) bool {
    if _, ok := w.ancestors[name]; ok {
        return false  // 祖先链中已有 → 阻止递归（cycle detection）
    }
    if w.ancestors == nil {
        w.ancestors = make(map[protoreflect.FullName]struct{})
    }
    w.ancestors[name] = struct{}{}
    return true
}

func (w *jsonWalker) leave(name protoreflect.FullName) {
    delete(w.ancestors, name)
}

func (w *jsonWalker) walkMessage(path httprule.FieldPath, message protoreflect.MessageDescriptor, f jsonLeafWalkFunc) {
    if !w.enter(message.FullName()) {
        return
    }
    defer w.leave(message.FullName())
    for i := 0; i < message.Fields().Len(); i++ {
        field := message.Fields().Get(i)
        p := append(httprule.FieldPath{}, path...)
        p = append(p, string(field.Name()))
        switch {
        case !field.IsMap() && field.Kind() == protoreflect.MessageKind:
            if IsWellKnownType(field.Message()) {
                f(p, field)
            } else {
                w.walkMessage(p, field.Message(), f)
            }
        default:
            f(p, field)
        }
    }
}
```

---

## 4. 已实施详情

### 4.1 改动文件

| 文件 | 改动 |
|---|---|
| `internal/plugin/jsonleafwalk.go` | `seen` → `ancestors`；新增 `leave`；`walkMessage` 用 `defer leave` 配对 |
| `internal/plugin/jsonleafwalk_test.go` | 新增 3 个测试 |

### 4.2 新增测试

| 测试 | 覆盖 |
|---|---|
| `Test_walkJSONLeafFields_sameMessageTypeAtMultipleSiblingFields` | 兄弟字段同类型（评审描述的主场景）：`RouteRequest{source:Address, destination:Address}` 输出 4 个 leaf |
| `Test_walkJSONLeafFields_sameMessageTypeInSeparateBranches` | 菱形布局：`D{b:B{x:A}, c:C{y:A}}` 输出 `b.x.a` 和 `c.y.a` |
| `Test_walkJSONLeafFields_selfReferenceDoesNotLoop` | 自引用防回归：`TreeNode{child:TreeNode}` 输出仅 `["name"]`（child 停止递归），带 2s 超时防死循环 |

测试用 `protodesc.NewFile` + `descriptorpb` 程序化构造 message descriptor，避免依赖 `buf generate`。

### 4.3 验证结果

- `go test -count=1 ./internal/plugin/ -run Test_walkJSONLeafFields -v` — 3/3 PASS
- `go test -count=1 ./...` — 全包通过
- `go vet ./...` — 通过
- `gofmt -l` — 干净
- `buf generate` + `deno fmt` 后 `git diff examples/proto/gen` — 干净（现有示例无同类型多字段结构，输出无变化）

---

## 5. 测试矩阵

| # | 用例 | 期望 | 状态 |
|---|---|---|---|
| 1 | 兄弟字段同类型 | 4 个 leaf 全有 | PASS |
| 2 | 菱形布局 | 跨分支 leaf 全有 | PASS |
| 3 | 自引用（防回归）| 仅顶层 leaf，cycle 在第一层 child 截断 | PASS |
| 4 | 普通嵌套 message（无重复）| 全部递归（回归）| 通过 servicegen 现有测试间接覆盖 |
| 5 | 集成测试 golden diff | 无变化（示例无此结构）| PASS |

---

## 6. 风险

| 风险 | 缓解 |
|---|---|
| 行为变更对外部用户 | 旧行为是 bug，无用户应依赖 |
| 测试覆盖了所有 walker 用例 | 评审与现有 servicegen 集成测试已覆盖 query 生成主路径 |
| 性能影响 | `ancestors` 仍是 map 查找，与 `seen` 同复杂度；`defer leave` 一次额外调用可忽略 |

---

## 7. 非目标（YAGNI）

- 不重写 walker 为通用图遍历算法
- 不引入 max-depth 限制（路径栈已防无限递归）
- 不抽出公共 walkMessage + messageEnter/Leave 接口（仅一处使用）

---

## 8. 后续

- 评审 P0 #3 关闭
- 提交当前修改到 git（`git add internal/plugin/jsonleafwalk.go internal/plugin/jsonleafwalk_test.go`）
- 与 P0 #1（additional_bindings）、P0 #2（repeated message query）的修复/实施相互独立，可独立提交

---

## 9. 验收标准（已达成）

- [x] `go test -count=1 ./...` 全部通过
- [x] `go vet ./...` 通过
- [x] `gofmt -l` 无输出
- [x] 集成测试生成物 `git diff` 干净
- [x] 评审 P0 #3 描述的问题已根因定位并修复
- [x] 回归保护测试（自引用防死循环）已加入
- [ ] 提交至 git（待执行）
