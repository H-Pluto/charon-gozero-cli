package errors

import (
	stderrors "errors"
	"fmt"
)

// BizError 是可对外返回的业务错误。
//
// 它同时携带三层信息，排障时按此顺序读：
//   - Code：对外契约，客户端据此分支处理
//   - Msg：对外文案，已按语言本地化
//   - Cause：内部原因，只进日志，绝不序列化给客户端
type BizError struct {
	Code  Code
	Msg   string
	Cause error
}

// Error 实现 error 接口。带 Cause 时把原因串进来，
// 因为 Error() 主要用于日志，日志里需要完整因果链。
func (e *BizError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Msg, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Msg)
}

// Unwrap 让 errors.Is / errors.As 能穿透到内部原因。
func (e *BizError) Unwrap() error { return e.Cause }

// HTTPStatus 返回该错误对应的 HTTP 状态码。
func (e *BizError) HTTPStatus() int { return e.Code.HTTP() }

// New 用错误码的默认描述构造错误。
func New(code Code, cause error) *BizError {
	return &BizError{
		Code:  code,
		Msg:   code.Message(DefaultLang),
		Cause: cause,
	}
}

// Newf 用错误码的默认描述构造错误，并把 format 作为内部原因。
// 适合在参数校验处直接写 Newf(CodeInvalidParam, "name %q 超长", name)。
func Newf(code Code, format string, args ...any) *BizError {
	return &BizError{
		Code:  code,
		Msg:   code.Message(DefaultLang),
		Cause: fmt.Errorf(format, args...),
	}
}

// NewWithMsg 覆盖默认描述，用于需要把具体原因告诉用户的场景。
// 谨慎使用：Msg 会直接暴露给客户端，不要塞内部细节。
func NewWithMsg(code Code, msg string, cause error) *BizError {
	return &BizError{Code: code, Msg: msg, Cause: cause}
}

// From 把任意 error 规整成 *BizError。
//
// nil 返回 nil，便于 if err := From(err); err != nil 的写法。
// 已经是 *BizError 的原样返回，因此可以安全地多次经过中间件。
// 其余错误一律归为 CodeInternal，并把原错误落到 Cause —— 这是关键的
// 安全边界：非业务错误的具体内容不会泄漏给客户端。
func From(err error) *BizError {
	if err == nil {
		return nil
	}

	var biz *BizError
	if stderrors.As(err, &biz) {
		return biz
	}

	return New(CodeInternal, err)
}

// Wrap 把一个底层错误包装成指定错误码的业务错误。
//
// 与 From 的区别：From 把未知错误一律归为 CodeInternal，而 Wrap 让调用方
// 显式指定对外错误码。典型场景是 repo 返回 sql.ErrNoRows，service 层
// Wrap(err, CodeNotFound) 后对客户端就是 404 而不是 500。
//
// 若 err 已经是 *BizError，则保留其原有错误码与文案，只在 Cause 为空时
// 补上底层原因，避免把已经确定好的对外契约覆盖掉。
func Wrap(err error, code Code) *BizError {
	if err == nil {
		return nil
	}

	var biz *BizError
	if stderrors.As(err, &biz) {
		return biz
	}

	return New(code, err)
}

// Is 判断 err 链上是否存在指定错误码的业务错误。
func Is(err error, code Code) bool {
	biz := From(err)
	return biz != nil && biz.Code == code
}
