下面按优先级列出当前项目的优化方向。

  P0 正确性

  1. 修复 query 参数空值判断：0、false、空字符串需要按 proto JSON 语义明确处理，不能简单用 if (request.field)。
  2. 修复 map query 生成：当前 TS 类型是对象，但生成代码使用 .forEach()，运行时会崩。
  3. 修复 map query key/value 编码：应使用 Object.entries()，同时编码 key 和 value。
  4. 明确 repeated query 格式：当前重复生成 field=value，需要确认是否符合目标服务约定，并补测试。
  5. path 变量需要 encodeURIComponent 或明确不编码策略；现在直接拼接，包含空格、?、# 等字符会破坏 URL。
  6. path variable 中的 sub-template 语义需要更严谨处理，例如 {name=shippers/*} 目前直接插入整个 name，这是常见 AIP 用法可接受，但应明确并测试。
  7. client 生成也要跳过 streaming RPC。目前 interface 跳过 streaming，但 client 生成只看 HTTP annotation，存在不一致。
  8. jsonPathSegments 需要防御非法 field path，避免 field == nil 时 panic。
  9. body: "nested.field" 是否支持需要明确；当前实现只按单段 body 字段处理。

  P0 TypeScript 类型语义
  10. 64 位整数应重新评估：proto JSON 中 int64/uint64 通常是 string，当前映射为 number 有精度风险。
  13. google.protobuf.NullValue enum 被映射为 null 是合理的，但需要专门测试。
  14. oneof 当前只是多个 optional 字段，不能表达互斥关系；可考虑生成 union 或保留简单模式但文档说明限制。
  15. required/optional 字段策略需要统一文档：当前非 optional 字段生成 T | undefined，不是 ?，部分文档仍有旧示例。
  16. RequestHandler、RequestType 应 export，否则 README 中用户无法直接导入使用。

  P1 HTTP 功能完整性
  17. ~~支持 additional_bindings 的生成策略：可以生成多个 client 方法变体，或提供 binding selector。~~ 已闭环（2026-06-02）：first-match-wins binding selector，spec 在 `docs/superpowers/specs/2026-06-02-additional-bindings-selector-design.md`。
  18. 支持 google.api.http.custom 的测试用例，不只靠代码路径。
  19. 补充 PUT/PATCH/DELETE/POST/GET 全方法 golden 覆盖。
  20. 明确是否支持无前导 / 的输出 path；现在生成 v1/...，用户拼接 root URL 时容易出错。
  21. 支持 query 参数数组、嵌套消息、WKT、枚举的规范序列化，而不是统一 .toString()。
  22. FieldMask query 应确认 camelCase 逗号格式，不应被普通 .toString() 假设掩盖。
  23. Timestamp、Duration query/body 只做类型表达，不做运行时转换；文档应说明调用方需要传 JSON 格式字符串。

  P1 测试体系
  24. 为 internal/plugin 添加单元测试，直接构造 descriptor 或使用 test proto golden。
  25. 给 service client 生成添加 golden tests，覆盖 path、body、query、custom verb。
  26. 添加 TypeScript 编译检查，至少对生成示例跑 tsc --noEmit。
  27. 添加生成代码运行时测试，用 JS/TS 调用 client，断言 handler 收到的 path/method/body/meta。
  28. 添加 query 边界测试：0、false、空数组、map、嵌套字段、枚举、WKT。
  29. 添加 streaming skip 测试。
  30. 添加 invalid annotation 测试，验证错误信息可读。
  31. CI 中区分 unit、integration、typescript compile、buf lint。

  P1 文档与示例
  32. README 当前功能承诺偏强，特别是 “proper canonical JSON” 和 “all WKT”，需要和实际能力对齐。
  33. 文档中 RequestHandler 示例应匹配是否 export。
  34. fetch 示例应处理 Content-Type: application/json 和 body: null。
  35. 示例 root URL 拼接需要统一，避免 https://host + v1/... 或双斜杠问题。
  36. ~~additional_bindings 文档已说明未生成，但 README feature list 不应暗示完整 HTTP transcoding。~~ 已闭环（2026-06-02）：docs/code-generation.md + docs/protobuf-annotations.md 已更新到 first-match-wins 行为描述。
  37. docs/protobuf-annotations.md 中 field behavior 示例仍是 name?: string，应改成当前 name: string | undefined。
  38. 补一个“已知限制”文档，列出 oneof、int64、runtime conversion、additional bindings 等。

  P1 工程化
  39. 增加 GitHub Actions 或等价 CI；仓库目前没有 .github 工作流。
  40. make integration 依赖本地工具和 Go cache，建议支持 GOCACHE 落在可写目录，减少环境问题。
  41. mage Integration 可以主动检查 buf 是否存在，并给出清晰错误。
  42. tests/integration 当前假设目录位置，可改为通过 git rev-parse --show-toplevel 或更稳的路径查找。
  43. .goreleaser.yml 需要确认新版 GoReleaser 配置字段，archives.replacements 可能已过时。
  44. release 文档中架构说明要和实际 GoReleaser 输出一致。

  P2 架构与可维护性
  45. 将 service generation 拆分为 path、body、query、method binding 几个小组件，便于独立测试。
  46. 为 TS 表达式生成引入小型 AST/helper，减少字符串拼接错误。
  47. 为 proto field path lookup 做集中封装，统一返回错误而不是 panic。
  48. WKT 映射和 scalar 映射增加表驱动测试。
  49. query serialization 抽成可复用生成函数，避免 list/map/scalar 分支散落。
  50. 明确 package/type naming 的冲突策略；当前扁平化包名前缀可能有碰撞风险。
  51. 对生成输出排序做稳定性检查，避免 map/package 顺序导致非确定性输出。

  P2 用户体验
  52. 支持插件参数，例如 export_request_handler=true、path_prefix_slash=true、int64=string|number。
  53. 支持生成 ESM-friendly 类型导出风格。
  54. 支持只生成 types、不生成 client，或只生成 client。
  55. 支持自定义 RequestHandler request shape，例如 headers、query object、abort signal。
  56. 生成代码可减少 @ts-nocheck 依赖，逐步做到能被 TypeScript 严格检查。
  57. 错误信息可以带 service/method/path variable，方便定位调用问题。
