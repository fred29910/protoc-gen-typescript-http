# Map Query 生成修复设计文档

**日期**: 2026-05-23  
**项目**: protoc-gen-typescript-http  
**状态**: 已批准，待实现

---

## 1. 问题描述

`internal/plugin/servicegen.go` 的 `generateMethodQuery` 函数在处理 `map` 类型字段生成 query 参数时，调用了 `.forEach((value, key) => {...})`。

这是 **ES6 `Map`** 对象的方法签名，但生成的 TypeScript 类型（见 `type.go`）将 protobuf map 字段映射为 **plain object**：

```typescript
// type.go 生成的类型
mapStringString: { [key: string]: string } | undefined;
```

Plain object 没有 `.forEach()` 方法，运行时会直接抛出：

```
TypeError: request.mapStringString.forEach is not a function
```

此外，原来模板字符串中的 `[key]` 是字面量字符串，而非变量：

```typescript
// 当前（错误）生成
queryParams.push(`annotations[key]=${encodeURIComponent(value.toString())}`)
// 结果：annotations[key]=foo  ← "key" 是字面量，不是实际 map key
```

---

## 2. 根因分析

| 层面 | 实际情况 |
|---|---|
| TS 类型（`type.go`）| `{ [key: string]: V } \| undefined`（plain object）|
| 生成代码（`servicegen.go`）| `.forEach((value, key) => {...})`（ES6 Map API）|
| 模板字面量 | `` `field[key]=value` ``（`key` 是字面字符串，不是变量）|

问题出在 `generateMethodQuery` 的 `field.IsMap()` 分支，编写者将 protobuf map 与 ES6 Map 混淆。

---

## 3. 修复方案（已批准：方案 A）

**最小化修复**：用 `Object.entries()` 迭代 plain object，保持 `field[key]=value` 序列化格式不变。

### 3.1 生成器改动（`servicegen.go`）

**修复前**：
```go
case field.IsMap():
    f.P(t(4), "request.", jp, ".forEach((value, key) => {")
    f.P(t(5), "queryParams.push(`", jp, "[key]=${encodeURIComponent(value.toString())}`)")
    f.P(t(4), "})")
```

**修复后**：
```go
case field.IsMap():
    f.P(t(4), "Object.entries(request.", jp, ").forEach(([key, value]) => {")
    f.P(t(5), "queryParams.push(`", jp, "[${key}]=${encodeURIComponent(value.toString())}`)")
    f.P(t(4), "})")
```

### 3.2 生成结果对比

**修复前**（运行时崩溃）：
```typescript
if (request.annotations !== undefined && request.annotations !== null) {
  request.annotations.forEach((value, key) => {
    queryParams.push(`annotations[key]=${encodeURIComponent(value.toString())}`)
  })
}
// TypeError: request.annotations.forEach is not a function
// 且即使不崩溃，[key] 也是字面量
```

**修复后**（正确）：
```typescript
if (request.annotations !== undefined && request.annotations !== null) {
  Object.entries(request.annotations).forEach(([key, value]) => {
    queryParams.push(`annotations[${key}]=${encodeURIComponent(value.toString())}`)
  })
}
// 正确输出：annotations[myKey]=myValue
```

---

## 4. 测试策略

### 4.1 Golden 测试（集成测试）

在 `examples/proto/einride/example/syntax/v1/syntax_service.proto` 中添加一个包含 map 字段 query 参数的 RPC 方法，让 map 字段出现在 query 路径中，触发生成器的 `IsMap()` 分支。

重新运行 `mage integration` 生成 golden 文件，验证生成代码的正确性。

> **注**：`examples/proto/einride/example/syntax/v1/syntax.proto` 中的 `Message` 已有 `map<string, string> map_string_string = 35`，但 `syntax_service.proto` 中的 `Request` 消息没有 map 字段，因此 map query 生成从未被集成测试覆盖过。

### 4.2 单元测试（可选）

为 `generateMethodQuery` 添加单元测试，直接验证 `field.IsMap()` 分支生成的代码字符串，不依赖 proto 编译。

---

## 5. 约束

- **只改 `servicegen.go` 的 `IsMap()` 分支**，不触碰其他逻辑
- **不修改 TS 类型定义**（`type.go`）
- **序列化格式保持 `field[key]=value`**（不改变格式，只修 API 调用方式）
- **不引入运行时 helper**，直接生成内联 `Object.entries()` 调用

---

## 6. 影响范围

| 文件 | 改动类型 |
|---|---|
| `internal/plugin/servicegen.go` | 修复（2 行字符串）|
| `examples/proto/einride/example/syntax/v1/syntax_service.proto` | 新增 RPC 方法（可选，用于覆盖测试）|
| `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` | 重新生成 golden（若添加测试 RPC）|

---

## 7. 成功标准

1. 含 map 字段的请求在运行时不再抛 `TypeError`
2. map 字段序列化为 `field[actualKey]=value` 格式
3. `mage test` 和 `mage integration` 全部通过
