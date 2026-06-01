package errcode

// Error codes as defined in dev doc §17.1
const (
	Success = 0

	// Auth errors (1xxx)
	Unauthenticated = 1001 // Token missing/invalid/expired
	Forbidden       = 1002 // Insufficient permissions
	StoreForbidden  = 1003 // Cross-store access denied

	// Client errors (2xxx)
	BadRequest = 2001 // Request parameter validation failed
	NotFound   = 2002 // Resource not found

	// Business logic errors (3xxx)
	StateTransitionInvalid = 3001 // Invalid state machine transition
	ResourceConflict       = 3002 // Resource time slot conflict
	InsufficientStock      = 3003 // Not enough inventory
	InsufficientWallet     = 3004 // Insufficient stored value balance

	// Payment errors (4xxx)
	PaymentNotEnabled = 4001 // Payment gateway not enabled

	// Server errors (5xxx)
	InternalError = 5000 // Internal server error
)

// Message returns the default Chinese message for a code.
func Message(code int) string {
	switch code {
	case Success:
		return "ok"
	case Unauthenticated:
		return "未认证或Token已失效"
	case Forbidden:
		return "无操作权限"
	case StoreForbidden:
		return "跨门店访问被拒"
	case BadRequest:
		return "参数校验失败"
	case NotFound:
		return "资源不存在"
	case StateTransitionInvalid:
		return "状态不可变更"
	case ResourceConflict:
		return "资源时段冲突"
	case InsufficientStock:
		return "库存不足"
	case InsufficientWallet:
		return "储值余额不足"
	case PaymentNotEnabled:
		return "线上支付未开通"
	case InternalError:
		return "服务器内部错误"
	default:
		return "未知错误"
	}
}
