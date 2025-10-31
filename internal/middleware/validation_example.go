package middleware

import (
	"net/http"

	"go-server/pkg/errors"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

// ExampleValidationMiddlewareUsage 演示如何在路由中使用验证中间件
func ExampleValidationMiddlewareUsage() *gin.Engine {
	router := gin.New()

	// 基础中间件
	router.Use(gin.Recovery())

	// ========== 登录和注册路由 ==========

	// 登录路由 - 使用登录验证配置
	router.POST("/auth/login", ValidationMiddleware(LoginValidation()), func(c *gin.Context) {
		// 获取验证后的数据
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		// 在这里处理登录逻辑
		// email := data["email"].(string)
		// password := data["password"].(string)

		response.Success(c, http.StatusOK, "Login validation passed", data)
	})

	// 注册路由 - 使用注册验证配置
	router.POST("/auth/register", ValidationMiddleware(RegisterValidation()), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		// 在这里处理注册逻辑
		// username := data["username"].(string)
		// email := data["email"].(string)
		// password := data["password"].(string)
		// firstName := data["first_name"].(string)
		// lastName := data["last_name"].(string)

		response.Success(c, http.StatusOK, "Registration validation passed", data)
	})

	// ========== 用户管理路由 ==========

	// 获取用户列表 - 使用分页验证
	router.GET("/users", ValidationMiddleware(PaginationValidation()), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		// 在这里处理获取用户列表逻辑
		// page := data["page"]
		// limit := data["limit"]

		response.Success(c, http.StatusOK, "Pagination validation passed", data)
	})

	// 更新用户 - 使用用户ID验证和更新用户验证配置
	router.PUT("/users/:id",
		ValidationMiddleware(UserIDValidation()),
		ValidationMiddleware(UpdateUserValidation()),
		func(c *gin.Context) {
			data, exists := GetValidatedData(c)
			if !exists {
				response.InternalServerError(c, "Failed to get validated data")
				return
			}

			// 从上下文中获取用户ID
			userID, exists := c.Get("user_id")
			if !exists {
				response.BadRequest(c, "User ID not found")
				return
			}

			// 在这里处理更新用户逻辑
			// id := userID.(string)
			// username := data["username"].(string)

			response.Success(c, http.StatusOK, "User update validation passed", gin.H{
				"user_id": userID,
				"data":    data,
			})
		})

	// ========== 密码管理路由 ==========

	// 修改密码 - 使用密码验证配置
	router.POST("/auth/change-password", ValidationMiddleware(ChangePasswordValidation()), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		// 在这里处理修改密码逻辑
		// oldPassword := data["old_password"].(string)
		// newPassword := data["new_password"].(string)

		response.Success(c, http.StatusOK, "Password change validation passed", data)
	})

	// ========== 自定义验证路由 ==========

	// 强密码验证示例
	router.POST("/auth/register-strong", ValidationMiddleware(StrongPasswordValidation()), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		response.Success(c, http.StatusOK, "Strong password validation passed", data)
	})

	// ========== 自定义验证规则示例 ==========

	// 自定义验证配置
	customValidation := ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "product_name",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 2},
					&MaxLengthRule{MaxLength: 100},
				},
			},
			{
				Field:    "price",
				Required: true,
				Rules: []ValidationRule{
					&MinRule{Min: 0},
					&MaxRule{Max: 10000},
				},
			},
			{
				Field:    "category",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 2},
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "description",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MaxLengthRule{MaxLength: 1000},
				},
			},
			{
				Field:    "image_url",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&URLRule{},
				},
			},
		},
		Validate: func(c *gin.Context) bool {
			// 可以在这里添加额外的业务逻辑验证
			// 例如：检查用户权限、验证数据关联等
			return true
		},
	}

	router.POST("/products", ValidationMiddleware(customValidation), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		response.Success(c, http.StatusCreated, "Product creation validation passed", data)
	})

	// ========== 错误处理示例 ==========

	// 演示各种验证错误的路由
	router.POST("/demo/errors", func(c *gin.Context) {
		// 这个路由不使用验证中间件，用于演示各种错误响应
		response.ValidationError(c, "Demo validation errors",
			errors.ErrorDetails{
				Field:      "email",
				Message:    "email must be a valid email address",
				Value:      "invalid-email",
				Constraint: "email_format",
			},
			errors.ErrorDetails{
				Field:      "password",
				Message:    "password must be at least 8 characters long",
				Value:      "123",
				Constraint: "min_length:8",
			},
		)
	})

	return router
}

// ExampleAdvancedValidationMiddlewareUsage 演示高级验证中间件使用
func ExampleAdvancedValidationMiddlewareUsage() *gin.Engine {
	router := gin.New()

	// ========== 基于结构体的验证 ==========

	// 使用结构体标签验证的路由
	router.POST("/users/struct", StructValidation(struct {
		Username  string `json:"username" binding:"required,min=3,max=50"`
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=6"`
		FirstName string `json:"first_name" binding:"max=50"`
		LastName  string `json:"last_name" binding:"max=50"`
	}{}), func(c *gin.Context) {
		model, exists := GetValidatedModel(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated model")
			return
		}

		response.Success(c, http.StatusOK, "Struct validation passed", model)
	})

	// ========== 组合验证中间件 ==========

	// 组合多个验证中间件
	router.POST("/users/complex",
		// 第一步：验证请求ID参数
		ValidationMiddleware(UserIDValidation()),
		// 第二步：验证基础字段
		ValidationMiddleware(UpdateUserValidation()),
		// 第三步：验证业务逻辑
		ValidationMiddleware(ValidationConfig{
			Validate: func(c *gin.Context) bool {
				// 这里可以添加额外的业务逻辑验证
				// 例如：检查用户是否有权限修改这个用户
				// 验证用户状态是否允许修改等

				// 模拟权限检查
				currentUserID, exists := c.Get("current_user_id")
				if !exists {
					response.Unauthorized(c, "User not authenticated")
					return false
				}

				targetUserID, exists := c.Get("user_id")
				if !exists {
					response.BadRequest(c, "Target user ID not found")
					return false
				}

				// 简单的权限检查：只能修改自己的信息
				if currentUserID != targetUserID {
					response.Forbidden(c, "You can only update your own profile")
					return false
				}

				return true
			},
		}),
		func(c *gin.Context) {
			data, exists := GetValidatedData(c)
			if !exists {
				response.InternalServerError(c, "Failed to get validated data")
				return
			}

			response.Success(c, http.StatusOK, "Complex validation passed", data)
		})

	// ========== 条件验证示例 ==========

	// 条件验证：根据某些字段值决定其他字段的验证规则
	router.POST("/orders", ValidationMiddleware(ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "order_type",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 1},
					&MaxLengthRule{MaxLength: 20},
				},
			},
			{
				Field:    "customer_id",
				Required: true,
				Rules: []ValidationRule{
					&RequiredRule{},
				},
			},
		},
		Validate: func(c *gin.Context) bool {
			data, exists := GetValidatedData(c)
			if !exists {
				return false
			}

			orderType, ok := data["order_type"].(string)
			if !ok {
				response.BadRequest(c, "Invalid order type")
				return false
			}

			// 根据订单类型进行条件验证
			switch orderType {
			case "online":
				// 在线订单需要邮箱
				if email, exists := data["email"]; !exists || email == "" {
					response.ValidationError(c, "Email is required for online orders",
						errors.ErrorDetails{
							Field:      "email",
							Message:    "Email is required for online orders",
							Value:      nil,
							Constraint: "conditional_required",
						})
					return false
				}
			case "pickup":
				// 自提订单需要门店ID
				if storeId, exists := data["store_id"]; !exists || storeId == "" {
					response.ValidationError(c, "Store ID is required for pickup orders",
						errors.ErrorDetails{
							Field:      "store_id",
							Message:    "Store ID is required for pickup orders",
							Value:      nil,
							Constraint: "conditional_required",
						})
					return false
				}
			}

			return true
		},
	}), func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if !exists {
			response.InternalServerError(c, "Failed to get validated data")
			return
		}

		response.Success(c, http.StatusCreated, "Order validation passed", data)
	})

	return router
}

// ExampleValidationInHandlers 演示如何在现有的handlers中使用验证中间件
func ExampleValidationInHandlers() {
	// 这个函数演示如何将验证中间件集成到现有的handlers中
	// 实际使用时，可以修改现有的路由配置

	// 原来的路由可能像这样：
	// router.POST("/auth/login", authHandler.Login)
	// router.POST("/auth/register", authHandler.Register)

	// 使用验证中间件后的路由：
	// router.POST("/auth/login", ValidationMiddleware(LoginValidation()), authHandler.Login)
	// router.POST("/auth/register", ValidationMiddleware(RegisterValidation()), authHandler.Register)

	// 然后在handler中获取验证后的数据：
	/*
		func (h *AuthHandler) Login(c *gin.Context) {
			data, exists := GetValidatedData(c)
			if !exists {
				response.InternalServerError(c, "Failed to get validated data")
				return
			}

			// 构建请求结构
			req := models.LoginRequest{
				Email:    data["email"].(string),
				Password: data["password"].(string),
			}

			// 继续原有的业务逻辑...
		}
	*/
}
