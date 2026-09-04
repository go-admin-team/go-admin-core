package runtime

import "github.com/gin-gonic/gin"

// Well-known keys the host registers under with SetMiddleware. An
// application that wants to reuse the host's own authentication and
// authorization chain - rather than build its own - looks these up through
// GetHandlerFunc instead of importing the host package that built them.
//
// All three keys are expected to store a gin.HandlerFunc. That is a change
// from an earlier host convention where the JWT key stored an unbound method
// expression: a value with no receiver bound to it, which a caller cannot
// turn into a working gin.HandlerFunc no matter how it type-asserts. Hosts
// must register a bound closure (for example authMiddleware.MiddlewareFunc(),
// not (*jwt.GinJWTMiddleware).MiddlewareFunc) so all three keys have the same
// shape and GetHandlerFunc works uniformly across them.
const (
	JwtTokenCheck   = "JwtToken"
	RoleCheck       = "AuthCheckRole"
	PermissionCheck = "PermissionAction"
)

// GetHandlerFunc is GetMiddleware plus the type assertion every caller of it
// otherwise has to repeat, and a reported failure instead of a panic when the
// key was never registered or was registered with something other than a
// gin.HandlerFunc.
//
// The bool result exists because a bare type assertion turns two different,
// unrelated problems - "not registered yet" and "registered with the wrong
// shape" - into a panic at the call site, typically inside a router
// initialiser that runs behind this framework's panic guard (see
// docs/contract.md section 4), which then reports the module as having
// failed to register any routes at all rather than naming the real cause.
// Callers that need JWT enforcement to be non-optional should still check ok
// explicitly and fail loudly, since a missing middleware is not a state a
// router should silently continue past.
func (e *Application) GetHandlerFunc(key string) (gin.HandlerFunc, bool) {
	h, ok := e.GetMiddleware(key).(gin.HandlerFunc)
	return h, ok
}
