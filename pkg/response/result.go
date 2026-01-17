package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppHandlerFunc func(c *gin.Context) (any, error)

// Wrap 是一个 Gin 中间件，用于包装 AppHandlerFunc 函数。
// 它会调用 AppHandlerFunc 函数，处理返回的数据和错误。
// 如果 AppHandlerFunc 返回错误，会根据错误类型返回不同的 HTTP 状态码和错误消息。
// 如果 AppHandlerFunc 返回数据，会自动将数据包装为 JSON 格式并返回。
// Example:
//
//	func GetUser(c *gin.Context) (any, error) {
//		id := c.Param("id")
//
//		if id == "0" {
//			return nil, app.NotFound("user not found")
//		}
//
//		if id == "x" {
//			return nil, app.BadRequest("invalid id format")
//		}
//
//		// 模拟未知错误
//		if id == "999" {
//			return nil, app.Internal(errors.New("db connection failed"))
//		}
//
//		return User{ID: 1, Name: "Tom"}, nil
//	}
func Wrap(fn AppHandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fn(c)

		if err != nil {
			var appErr *AppError
			if errors.As(err, &appErr) {
				c.String(appErr.Status, appErr.Msg)
				return
			}
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, data)
	}
}
