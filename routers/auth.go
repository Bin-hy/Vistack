package routers

import (
    "net/http"

    "github.com/binhy/vistack/core"
    "github.com/binhy/vistack/models"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type loginPayload struct {
    Account  string `json:"account" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type registerPayload struct {
    Account  string `json:"account" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// RegisterAuth 认证相关路由
func RegisterAuth(g *gin.RouterGroup) {
    ag := g.Group("/auth")

    // 登录（示例实现，实际应校验密码并签发 JWT）
    ag.POST("/login", func(c *gin.Context) {
        var p loginPayload
        if err := c.ShouldBindJSON(&p); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误", "details": err.Error()})
            return
        }
        user := models.User{Account: p.Account, Nickname: p.Account}
        token := uuid.NewString()
        c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
    })

    // 注册（示例：如果已配置数据库，则落库）
    ag.POST("/register", func(c *gin.Context) {
        var p registerPayload
        if err := c.ShouldBindJSON(&p); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误", "details": err.Error()})
            return
        }

        user := models.User{Account: p.Account, Nickname: p.Account}
        if core.DB != nil {
            if err := core.DB.Create(&user).Error; err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败", "details": err.Error()})
                return
            }
        }
        c.JSON(http.StatusOK, gin.H{"user": user})
    })
}