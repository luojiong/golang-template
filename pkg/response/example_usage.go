package response

import (
	"go-server/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ExampleUsage 演示增强响应包的使用方法
func ExampleUsage() {
	// 创建Gin路由器
	r := gin.Default()

	// 添加关联ID中间件
	r.Use(WithCorrelationID())

	// 示例1: 成功响应
	r.GET("/success", func(c *gin.Context) {
		data := map[string]interface{}{
			"user_id": 123,
			"name":    "张三",
			"email":   "zhangsan@example.com",
		}
		SuccessWithData(c, data)
	})

	// 示例2: 验证错误响应
	r.POST("/validate", func(c *gin.Context) {
		// 模拟验证失败
		ValidationError(c, "输入数据验证失败",
			errors.ErrorDetails{
				Field:      "email",
				Message:    "邮箱格式不正确",
				Value:      "invalid-email",
				Constraint: "必须是有效的邮箱格式",
			},
			errors.ErrorDetails{
				Field:      "age",
				Message:    "年龄必须大于18岁",
				Value:      16,
				Constraint: "min: 18",
			},
		)
	})

	// 示例3: 资源未找到错误
	r.GET("/users/:id", func(c *gin.Context) {
		userID := c.Param("id")
		NotFoundError(c, "用户", userID)
	})

	// 示例4: 数据库错误
	r.GET("/db-error", func(c *gin.Context) {
		DatabaseError(c, "连接数据库失败",
			errors.NewAppError(errors.ErrCodeDatabase, "connection timeout"))
	})

	// 示例5: 使用AppError直接响应
	r.GET("/app-error", func(c *gin.Context) {
		appError := errors.NewRateLimitError(100, 3600).
			WithCorrelationID("custom-correlation-id").
			WithUserMessage("请求过于频繁，请稍后再试").
			WithDetail("retry_after", 3600)
		ErrorWithAppError(c, appError)
	})

	// 示例6: 包装现有错误
	r.GET("/wrap-error", func(c *gin.Context) {
		originalErr := errors.NewAppError(errors.ErrCodeValidation, "密码格式错误")
		WrapError(c, originalErr, errors.ErrCodeValidation, "用户输入验证失败")
	})

	// 启动服务器
	r.Run(":8080")
}

// ResponseFormatExamples 展示各种响应格式示例
func ResponseFormatExamples() {
	// 1. 成功响应格式
	/*
		{
			"success": true,
			"message": "Success",
			"data": {
				"user_id": 123,
				"name": "张三"
			},
			"correlation_id": "abc12345",
			"timestamp": "2025-01-15T10:30:00Z"
		}
	*/

	// 2. 验证错误响应格式
	/*
		{
			"success": false,
			"message": "输入数据验证失败",
			"error": {
				"code": "VALIDATION_ERROR",
				"message": "输入数据验证失败",
				"user_message": "请检查输入数据的格式",
				"details": {
					"validation_errors": [
						{
							"field": "email",
							"message": "邮箱格式不正确",
							"value": "invalid-email",
							"constraint": "必须是有效的邮箱格式"
						}
					]
				},
				"internal_error": "validation failed: email format invalid"
			},
			"correlation_id": "def67890",
			"timestamp": "2025-01-15T10:31:00Z"
		}
	*/

	// 3. 资源未找到错误响应格式
	/*
		{
			"success": false,
			"message": "用户 with identifier '456' not found",
			"error": {
				"code": "NOT_FOUND",
				"message": "用户 with identifier '456' not found",
				"user_message": "请求的用户不存在",
				"details": {
					"resource_type": "用户",
					"identifier": "456"
				}
			},
			"correlation_id": "ghi11111",
			"timestamp": "2025-01-15T10:32:00Z"
		}
	*/
}

// 在实际项目中的使用建议：
// 1. 在路由设置中全局添加WithCorrelationID中间件
// 2. 优先使用具体的错误函数（ValidationError, NotFoundError等）
// 3. 对于复杂错误，直接使用ErrorWithAppError函数
// 4. 在开发环境检查internal_error字段进行调试
// 5. 使用correlation_id进行请求追踪和问题定位
