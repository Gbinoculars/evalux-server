package router

import (
	"evalux-server/ent"
	"evalux-server/internal/config"
	"evalux-server/internal/handler"
	"evalux-server/internal/llm"
	"evalux-server/internal/middleware"
	"evalux-server/internal/repo"
	"evalux-server/internal/service"
	"evalux-server/internal/storage"

	"github.com/gin-gonic/gin"
)

func Setup(client *ent.Client, cfg *config.Config, cloudreveStorage *storage.CloudreveStorage) *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 256 << 20 // 256MB，支持大录屏文件上传
	r.Use(middleware.CORS())

	// ==================== 依赖注入 ====================
	userRepo := repo.NewUserRepo(client)
	unifiedPermRepo := repo.NewUnifiedPermRepo(client)
	projectRepo := repo.NewProjectRepo(client)
	orgRepo := repo.NewOrgRepo(client)
	invitationRepo := repo.NewInvitationRepo(client)
	profileRepo := repo.NewProfileRepo(client)
	taskRepo := repo.NewTaskRepo(client)
	questionnaireRepo := repo.NewQuestionnaireRepo(client)
	executionRepo := repo.NewExecutionRepo(client)
	resultRepo := repo.NewResultRepo(client)
	planRepo := repo.NewPlanRepo(client)
	executionRunRepo := repo.NewExecutionRunRepo(client)
	promptRepo := repo.NewPromptRepo(client)
	llmClient := llm.NewClient()

	promptService := service.NewPromptService(promptRepo, unifiedPermRepo)
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo, unifiedPermRepo)
	projectService := service.NewProjectService(projectRepo, unifiedPermRepo)
	orgService := service.NewOrgService(orgRepo, unifiedPermRepo)
	invitationService := service.NewInvitationService(invitationRepo, unifiedPermRepo)
	profileService := service.NewProfileService(profileRepo, projectRepo, unifiedPermRepo, llmClient, promptService)
	taskService := service.NewTaskService(taskRepo, unifiedPermRepo)
	questionnaireService := service.NewQuestionnaireServiceWithLLM(questionnaireRepo, unifiedPermRepo, projectRepo, llmClient, promptService)
	executionService := service.NewExecutionService(client, executionRepo, taskRepo, profileRepo, projectRepo, executionRunRepo, resultRepo, unifiedPermRepo, llmClient, cloudreveStorage, promptService)
	resultService := service.NewResultService(resultRepo, executionRepo, taskRepo, profileRepo, projectRepo, questionnaireRepo, unifiedPermRepo, llmClient, promptService)
	planService := service.NewPlanService(planRepo, executionRepo, unifiedPermRepo)
	planService.SetRunRepo(executionRunRepo)
	deviceService := service.NewDeviceService()

	authHandler := handler.NewAuthHandler(authService, cfg)
	userHandler := handler.NewUserHandler(userService)
	projectHandler := handler.NewProjectHandler(projectService)
	orgHandler := handler.NewOrgHandler(orgService)
	invitationHandler := handler.NewInvitationHandler(invitationService)
	profileHandler := handler.NewProfileHandler(profileService)
	taskHandler := handler.NewTaskHandler(taskService)
	questionnaireHandler := handler.NewQuestionnaireHandler(questionnaireService)
	executionHandler := handler.NewExecutionHandler(executionService)
	resultHandler := handler.NewResultHandler(resultService)
	deviceHandler := handler.NewDeviceHandler(deviceService)
	planHandler := handler.NewPlanHandler(planService, executionService, executionRunRepo)
	promptHandler := handler.NewPromptHandler(promptService)

	// ==================== 公开接口 ====================
	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}
	}

	// ==================== 需要登录的接口 ====================
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(cfg))
	{
		protected.GET("/auth/me", authHandler.GetMe)

		// 用户管理
		users := protected.Group("/users")
		{
			users.GET("", userHandler.List)
			users.GET("/:id", userHandler.GetByID)
			users.POST("", userHandler.Create)
			users.PUT("/:id", userHandler.Update)
			users.PUT("/:id/status", userHandler.SetStatus)
			users.PUT("/:id/password", userHandler.ResetPassword)
			users.POST("/:id/roles", userHandler.AssignRole)
			users.DELETE("/:id/roles", userHandler.RemoveRole)
		}

		// 组织管理
		orgs := protected.Group("/orgs")
		{
			orgs.GET("/roles", orgHandler.ListRoles)
			orgs.GET("/mine", orgHandler.ListMine)
			orgs.GET("", orgHandler.List)
			orgs.POST("", orgHandler.Create)
			orgs.GET("/:id", orgHandler.GetByID)
			orgs.PUT("/:id", orgHandler.Update)
			orgs.DELETE("/:id", orgHandler.Delete)
			orgs.GET("/:id/children", orgHandler.ListChildren)
			// 组织成员管理
			orgs.GET("/:id/members", orgHandler.ListMembers)
			orgs.POST("/:id/members", orgHandler.AddMember)
			orgs.PUT("/:id/members/:userId", orgHandler.UpdateMemberRole)
			orgs.DELETE("/:id/members/:userId", orgHandler.RemoveMember)
		}

		// 项目管理
		projects := protected.Group("/projects")
		{
			projects.GET("/roles", projectHandler.ListRoles)
			projects.GET("", projectHandler.List)
			projects.POST("", projectHandler.Create)
			projects.GET("/:id", projectHandler.GetByID)
			projects.PUT("/:id", projectHandler.Update)
			projects.DELETE("/:id", projectHandler.Delete)

			// 项目成员管理
			projects.GET("/:id/members", projectHandler.ListMembers)
			projects.POST("/:id/members", projectHandler.AddMember)
			projects.PUT("/:id/members/:userId", projectHandler.UpdateMemberRole)
			projects.DELETE("/:id/members/:userId", projectHandler.RemoveMember)

			// 项目邀请
			projects.POST("/:id/invitations", invitationHandler.Invite)
			projects.GET("/:id/invitations", invitationHandler.ListByProject)

			// 项目下的子资源
			projects.GET("/:id/profiles", profileHandler.List)
			projects.GET("/:id/tasks", taskHandler.List)
			projects.GET("/:id/questionnaires", questionnaireHandler.ListTemplates)
			projects.POST("/:id/questionnaires/ai-generate", questionnaireHandler.AIGenerateQuestionnaire)
			projects.GET("/:id/executions", executionHandler.ListSessions)
			projects.GET("/:id/plans", planHandler.List)
			projects.GET("/:id/execution-runs", planHandler.ListExecutionRuns)
			projects.GET("/:id/results/overview", resultHandler.GetProjectOverview)
			projects.POST("/:id/results/snapshot", resultHandler.GenerateSnapshot)
			projects.POST("/:id/answers/batch", resultHandler.BatchGetAnswers)
			projects.GET("/:id/results/report/latest", resultHandler.GetLatestAIReport)
			projects.GET("/:id/results/report/html/latest", resultHandler.GetLatestHTMLReport)

			// 项目提示词管理
			projects.GET("/:id/prompts", promptHandler.List)
			projects.PUT("/:id/prompts", promptHandler.Update)
			projects.POST("/:id/prompts/reset", promptHandler.Reset)
		}

		// 以 run 为入口的报告 API
		runs := protected.Group("/runs")
		{
			runs.POST("/:runId/stats", resultHandler.GetProjectReportStats)
			runs.POST("/:runId/report", resultHandler.GetProjectReport)
			runs.POST("/:runId/report/html", resultHandler.GenerateHTMLReport)
		}

		// 全局邀请操作（当前用户视角）
		invitations := protected.Group("/invitations")
		{
			invitations.GET("/pending", invitationHandler.ListMyPending)
			invitations.POST("/:id/respond", invitationHandler.Respond)
		}

		// 画像管理
		profiles := protected.Group("/profiles")
		{
			profiles.POST("/generate", profileHandler.Generate)
			profiles.POST("/generate-stream", profileHandler.GenerateStream)
			profiles.GET("/generate-stream", profileHandler.GenerateStreamGET)
			profiles.POST("/batch-delete", profileHandler.BatchDelete)
			profiles.GET("/:id", profileHandler.GetByID)
			profiles.PUT("/:id", profileHandler.Update)
			profiles.DELETE("/:id", profileHandler.Delete)
		}

		// 任务管理（任务退化为纯任务定义，绑定上提到 plan）
		tasks := protected.Group("/tasks")
		{
			tasks.POST("", taskHandler.Create)
			tasks.GET("/:id", taskHandler.GetByID)
			tasks.PUT("/:id", taskHandler.Update)
			tasks.DELETE("/:id", taskHandler.Delete)
		}

		// 问卷管理
		questionnaires := protected.Group("/questionnaires")
		{
			questionnaires.POST("", questionnaireHandler.CreateTemplate)
			questionnaires.GET("/:id", questionnaireHandler.GetTemplate)
			questionnaires.PUT("/:id", questionnaireHandler.UpdateTemplate)
			questionnaires.DELETE("/:id", questionnaireHandler.DeleteTemplate)
			questionnaires.POST("/:id/questions", questionnaireHandler.CreateQuestion)
			questionnaires.GET("/:id/questions", questionnaireHandler.ListQuestions)
			questionnaires.POST("/:id/reorder", questionnaireHandler.ReorderQuestions)
		}
		protected.DELETE("/questions/:id", questionnaireHandler.DeleteQuestion)
		protected.PUT("/questions/:id", questionnaireHandler.UpdateQuestion)

		// 评估执行
		executions := protected.Group("/executions")
		{
			executions.POST("/start", executionHandler.StartSession)
			executions.POST("/report-step", executionHandler.ReportStep)
			executions.POST("/finish", executionHandler.FinishSession)
			executions.GET("/:id", executionHandler.GetSession)
			executions.GET("/:id/steps", executionHandler.ListSteps)
			executions.POST("/:id/screenshot", executionHandler.UploadStepScreenshot)
			executions.POST("/:id/recording", executionHandler.UploadRecording)
			executions.GET("/:id/result", resultHandler.GetSessionResult)
		}

		// 文件访问（Cloudreve 签发临时 URL，前端直连下载）
		files := protected.Group("/files")
		{
			files.POST("/url", executionHandler.GetFileURL)
			files.GET("/url", executionHandler.GetFileURLQuery)
			files.GET("/proxy", executionHandler.ProxyFile)
		}

		// 结果分析
		results := protected.Group("/results")
		{
			results.POST("/generate-eval", resultHandler.GenerateEvaluation)
			results.POST("/generate-questionnaire", resultHandler.GenerateQuestionnaireAnswers)
		}

		// 设备管理
		devices := protected.Group("/devices")
		{
			devices.GET("", deviceHandler.ListDevices)
		}

		// 执行计划管理
		plans := protected.Group("/plans")
		{
			plans.POST("", planHandler.Create)
			plans.GET("/:id", planHandler.GetByID)
			plans.PUT("/:id", planHandler.Update)
			plans.DELETE("/:id", planHandler.Delete)
			plans.POST("/abtest-result", planHandler.GetABTestResult)
			plans.POST("/:id/start", planHandler.StartRun)
		}

		// 执行运行记录
		protected.GET("/execution-runs/:id", planHandler.GetExecutionRun)
		protected.POST("/execution-runs/:id/abort", planHandler.AbortRun)
	}

	return r
}
