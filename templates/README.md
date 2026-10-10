# goctl 定制模板

本目录存放 charon 的 goctl 代码生成模板，让生成的 handler / logic 直接符合本项目的约定，而不是 goctl 的默认样板。

约定来源是 `internal/handler/` 与 `internal/logic/` 下的**手写实现**——它们才是真源，模板只是让它俩在将来切回自动生成时少改几行。`cmd/api/main.go` 顶部也写了同一句话：templates 仅供参考，以运行时代码为准。

## 当前状态（重要）

**这些模板目前没有接入任何自动流程，`make gen` 也不读取它们。**

- `internal/handler`、`internal/logic`、`internal/types` 现在是**手写维护**的（见 `Makefile` 的 `gen` 目标注释）。
- `make gen` 只是把 goctl 的**默认**输出生成到 `.gen/`，供人工对比，不会覆盖手写文件，也不会用到本目录。
- 也就是说：改了 `.api` 之后，**当前仍需手工**同步 handler / logic / routes。本目录的价值在于「将来切换」时有一套已对齐项目风格的模板可用。

## 与项目约定的对齐点

`templates/api/handler.tpl`

- handler 只做「解析请求 → 调 logic → 写响应」，不写业务判断。
- 解析失败走 `response.Fail(r.Context(), w, err)`，成功走 `response.Ok`，而不是 goctl 默认的 `httpx.ErrorCtx` / `httpx.OkJsonCtx`——统一响应外壳由 `pkg/response` 决定，业务代码不直接碰 `httpx` 的响应函数。
- 无返回值的接口用 `response.Ok(r.Context(), w, nil)`。

`templates/api/logic.tpl`

- logic 结构体内嵌 `logx.Logger`，持有 `ctx` 与 `svcCtx`，构造函数 `New<X>Logic(ctx, svcCtx)`——与手写实现一致（`logx.WithContext` 把 trace 带进日志）。
- 注释里强调用 `charon/pkg/errors`（`bizerr.NewFromContext` / `bizerr.Newf`）构造错误，不要返回裸 error。

模板里用到的变量（`{{.HandlerName}}`、`{{.LogicType}}`、`{{.Call}}`、`{{.ImportPackages}}`、`{{.function}}`、`{{.request}}`、`{{.responseType}}`、`{{.returnString}}` 等）都是 goctl v1.10.2 `api go` 生成器提供的，与默认模板同源，不要臆造变量名。

## 怎么用

goctl 通过 `--home` 指定模板根目录，它会去 `<home>/{version}/<category>/` 找模板文件。本项目对应的 category 是 `api`，模板文件名必须是 goctl 认识的名字（`handler.tpl`、`logic.tpl`）。

方式一：临时把模板拷进 goctl 的默认 home，再生成（推荐，改动最小）

```bash
# goctl 默认 home 是 ~/.goctl；api 类模板位于 <home>/<version>/api/
VERSION=$(goctl --version | awk '{print $NF}')   # 例如 v1.10.2
HOME_DIR="$HOME/.goctl/$VERSION/api"
mkdir -p "$HOME_DIR"
cp templates/api/handler.tpl templates/api/logic.tpl "$HOME_DIR/"

goctl api go -api api/desc/charon.api -dir .gen --style goZero
```

方式二：用 `--home` 直接指向一个独立目录（不改动全局 goctl home）

```bash
# 需要先摆好 <home>/<version>/api/ 的层级，模板才找得到
mkdir -p .tmpl-home/v1.10.2/api
cp templates/api/*.tpl .tmpl-home/v1.10.2/api/

goctl api go -api api/desc/charon.api -dir .gen --style goZero --home .tmpl-home
```

> `--style goZero` 与 `Makefile` 的 `gen` 目标保持一致；`.gen/` 已在 `.gitignore` / `make clean` 覆盖范围内，用完即弃。

## `.in` 约定（**尚未启用**）

goctl 生成的文件一旦被手工改过，下次生成就会覆盖掉手工改动。常见对策是维护一份「手工版」副本：

- 对每个会被反复重生成、但又要保留手工逻辑的文件，手工维护一个同名 `.in` 副本——例如 `internal/logic/hello/hello.go.in`。
- 生成前先由 `.in` 恢复出目标文件（`cp x.go.in x.go`），生成完再把 goctl 写进 `x.go` 的样板与 `.in` 里的手工逻辑合并回去。

**这套约定在本仓库目前没有生效，也没有任何脚本实现它。** 现状是：

- 没有 `.in` 文件，`make gen` 也不做「先恢复再生成」。
- 生成目录 `internal/` 下的文件仍以手写为准。
- 启用 `.in` 约定是一次**独立改动**：需要新增恢复/合并脚本、改造 `make gen`、并补一份操作说明。在它落地之前，请不要假设 `.in` 会自动生效，也不要指望生成会保住手工逻辑。

把它写在这里是为了记录「我们打算怎么解决生成与手改的冲突」，而不是宣称它已经可用。

## 目录结构

```
templates/
├── README.md            # 本文件
└── api/
    ├── handler.tpl      # goctl api go 的 handler 模板
    └── logic.tpl        # goctl api go 的 logic 模板
```
