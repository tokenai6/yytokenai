package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// Recovery 全局异常捕获中间件
func Recovery(r *ghttp.Request) {
	defer func() {
		if err := recover(); err != nil {
			g.Log().Error(r.Context(), "panic recovered:", err)

			// 返回统一的错误响应
			r.Response.WriteJson(g.Map{
				"code": 500,
				"msg":  "sys error",
				"data": nil,
			})
			r.Exit()
		}
	}()

	r.Middleware.Next()
}
