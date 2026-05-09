package repo

import (
	"context"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxexecutionplan"
	"evalux-server/ent/uxplanmodelconfig"
	"evalux-server/ent/uxplanprofilebinding"
	"evalux-server/ent/uxplanprofilequestionnairebinding"
	"evalux-server/ent/uxplantaskbinding"
	"evalux-server/ent/uxplantaskquestionnairebinding"
	"evalux-server/ent/uxquestionnairetemplate"
	"evalux-server/ent/uxtask"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type PlanRepo struct {
	client *ent.Client
}

func NewPlanRepo(client *ent.Client) *PlanRepo {
	return &PlanRepo{client: client}
}

// Create 创建执行计划（含全部四类绑定与模型配置，单次事务）
func (r *PlanRepo) Create(ctx context.Context, createdBy string, req model.CreatePlanRequest) (*model.PlanDetail, error) {
	uid, err := uuid.Parse(createdBy)
	if err != nil {
		return nil, err
	}
	pid, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return nil, err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	planBuilder := tx.UxExecutionPlan.Create().
		SetProjectID(pid).
		SetPlanName(req.PlanName).
		SetPlanType(uxexecutionplan.PlanType(req.PlanType)).
		SetMaxConcurrency(maxOr(req.MaxConcurrency, 1)).
		SetStepTimeoutSec(maxOr(req.StepTimeoutSec, 60)).
		SetSessionTimeoutSec(maxOr(req.SessionTimeoutSec, 300)).
		SetRetryLimit(maxOr(req.RetryLimit, 3)).
		SetStatus(uxexecutionplan.StatusREADY).
		SetCreatedBy(uid)
	if req.Hypothesis != "" {
		planBuilder = planBuilder.SetHypothesis(req.Hypothesis)
	}
	if req.PromptOverrideID != "" {
		if pou, err := uuid.Parse(req.PromptOverrideID); err == nil {
			planBuilder = planBuilder.SetPromptOverrideID(pou)
		}
	}
	plan, err := planBuilder.Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := writePlanBindings(ctx, tx, plan.ID, req.ModelConfigs, req.TaskBindings, req.ProfileBindings, req.TaskQuestionnaireBindings, req.ProfileQuestionnaireBindings); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, plan.ID.String())
}

// GetByID 取计划详情，连带四类绑定与模型配置
func (r *PlanRepo) GetByID(ctx context.Context, planID string) (*model.PlanDetail, error) {
	pid, err := uuid.Parse(planID)
	if err != nil {
		return nil, err
	}
	p, err := r.client.UxExecutionPlan.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	d := entPlanToDetail(p)
	if err := r.fillPlanBindings(ctx, p.ID, d); err != nil {
		return nil, err
	}
	return d, nil
}

// ListByProject 列出项目下所有可用计划（含子绑定与模型配置）
func (r *PlanRepo) ListByProject(ctx context.Context, projectID string) ([]*model.PlanDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.UxExecutionPlan.Query().
		Where(uxexecutionplan.ProjectID(pid), uxexecutionplan.StatusEQ(uxexecutionplan.StatusREADY)).
		Order(ent.Desc(uxexecutionplan.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*model.PlanDetail, 0, len(rows))
	for _, p := range rows {
		d := entPlanToDetail(p)
		_ = r.fillPlanBindings(ctx, p.ID, d)
		result = append(result, d)
	}
	return result, nil
}

// Update 更新计划主表 + 四类绑定（"覆盖式"，传入即视作完整集合）
func (r *PlanRepo) Update(ctx context.Context, planID string, req model.UpdatePlanRequest) (*model.PlanDetail, error) {
	pid, err := uuid.Parse(planID)
	if err != nil {
		return nil, err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	upd := tx.UxExecutionPlan.UpdateOneID(pid).SetUpdatedAt(time.Now())
	if req.PlanName != nil {
		upd = upd.SetPlanName(*req.PlanName)
	}
	if req.MaxConcurrency != nil {
		upd = upd.SetMaxConcurrency(*req.MaxConcurrency)
	}
	if req.StepTimeoutSec != nil {
		upd = upd.SetStepTimeoutSec(*req.StepTimeoutSec)
	}
	if req.SessionTimeoutSec != nil {
		upd = upd.SetSessionTimeoutSec(*req.SessionTimeoutSec)
	}
	if req.RetryLimit != nil {
		upd = upd.SetRetryLimit(*req.RetryLimit)
	}
	if req.Hypothesis != nil {
		if *req.Hypothesis == "" {
			upd = upd.ClearHypothesis()
		} else {
			upd = upd.SetHypothesis(*req.Hypothesis)
		}
	}
	if req.PromptOverrideID != nil {
		if *req.PromptOverrideID == "" {
			upd = upd.ClearPromptOverrideID()
		} else if pou, err := uuid.Parse(*req.PromptOverrideID); err == nil {
			upd = upd.SetPromptOverrideID(pou)
		}
	}
	if req.Status != nil {
		upd = upd.SetStatus(uxexecutionplan.Status(*req.Status))
	}
	if _, err := upd.Save(ctx); err != nil {
		return nil, err
	}

	// 覆盖式更新绑定（仅当客户端传入时才动）
	if req.ModelConfigs != nil {
		if _, err := tx.UxPlanModelConfig.Delete().Where(uxplanmodelconfig.PlanID(pid)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if req.TaskBindings != nil {
		if _, err := tx.UxPlanTaskBinding.Delete().Where(uxplantaskbinding.PlanID(pid)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if req.ProfileBindings != nil {
		if _, err := tx.UxPlanProfileBinding.Delete().Where(uxplanprofilebinding.PlanID(pid)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if req.TaskQuestionnaireBindings != nil {
		if _, err := tx.UxPlanTaskQuestionnaireBinding.Delete().Where(uxplantaskquestionnairebinding.PlanID(pid)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if req.ProfileQuestionnaireBindings != nil {
		if _, err := tx.UxPlanProfileQuestionnaireBinding.Delete().Where(uxplanprofilequestionnairebinding.PlanID(pid)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if err := writePlanBindings(ctx, tx, pid, req.ModelConfigs, req.TaskBindings, req.ProfileBindings, req.TaskQuestionnaireBindings, req.ProfileQuestionnaireBindings); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, planID)
}

func (r *PlanRepo) Delete(ctx context.Context, planID string) error {
	pid, err := uuid.Parse(planID)
	if err != nil {
		return err
	}
	// 先清理所有关联的绑定表和模型配置（外键约束）
	_, _ = r.client.UxPlanModelConfig.Delete().Where(uxplanmodelconfig.PlanID(pid)).Exec(ctx)
	_, _ = r.client.UxPlanTaskBinding.Delete().Where(uxplantaskbinding.PlanID(pid)).Exec(ctx)
	_, _ = r.client.UxPlanProfileBinding.Delete().Where(uxplanprofilebinding.PlanID(pid)).Exec(ctx)
	_, _ = r.client.UxPlanTaskQuestionnaireBinding.Delete().Where(uxplantaskquestionnairebinding.PlanID(pid)).Exec(ctx)
	_, _ = r.client.UxPlanProfileQuestionnaireBinding.Delete().Where(uxplanprofilequestionnairebinding.PlanID(pid)).Exec(ctx)
	return r.client.UxExecutionPlan.DeleteOneID(pid).Exec(ctx)
}

// fillPlanBindings 把四类绑定 + 模型配置填充到 PlanDetail
func (r *PlanRepo) fillPlanBindings(ctx context.Context, planID uuid.UUID, d *model.PlanDetail) error {
	// 模型配置
	mcs, err := r.client.UxPlanModelConfig.Query().
		Where(uxplanmodelconfig.PlanID(planID)).All(ctx)
	if err != nil {
		return err
	}
	d.ModelConfigs = make([]model.PlanModelConfig, 0, len(mcs))
	for _, m := range mcs {
		d.ModelConfigs = append(d.ModelConfigs, entPlanModelToDTO(m))
	}

	// task bindings + 联表 task_name
	tbs, err := r.client.UxPlanTaskBinding.Query().
		Where(uxplantaskbinding.PlanID(planID)).
		Order(ent.Asc(uxplantaskbinding.FieldExecutionOrder)).
		All(ctx)
	if err != nil {
		return err
	}
	d.TaskBindings = make([]model.PlanTaskBindingDTO, 0, len(tbs))
	for _, b := range tbs {
		dto := model.PlanTaskBindingDTO{
			BindingID:      b.ID.String(),
			TaskID:         b.TaskID.String(),
			ExecutionOrder: b.ExecutionOrder,
			Enabled:        b.Enabled,
		}
		if t, e := r.client.UxTask.Query().Where(uxtask.ID(b.TaskID)).Only(ctx); e == nil {
			dto.TaskName = t.TaskName
		}
		d.TaskBindings = append(d.TaskBindings, dto)
	}

	pbs, err := r.client.UxPlanProfileBinding.Query().
		Where(uxplanprofilebinding.PlanID(planID)).
		Order(ent.Asc(uxplanprofilebinding.FieldExecutionOrder)).
		All(ctx)
	if err != nil {
		return err
	}
	d.ProfileBindings = make([]model.PlanProfileBindingDTO, 0, len(pbs))
	for _, b := range pbs {
		d.ProfileBindings = append(d.ProfileBindings, model.PlanProfileBindingDTO{
			BindingID:      b.ID.String(),
			ProfileID:      b.ProfileID.String(),
			ExecutionOrder: b.ExecutionOrder,
			Enabled:        b.Enabled,
		})
	}

	tqbs, err := r.client.UxPlanTaskQuestionnaireBinding.Query().
		Where(uxplantaskquestionnairebinding.PlanID(planID)).
		Order(ent.Asc(uxplantaskquestionnairebinding.FieldQuestionOrder)).
		All(ctx)
	if err != nil {
		return err
	}
	d.TaskQuestionnaireBindings = make([]model.PlanTaskQuestionnaireBindingDTO, 0, len(tqbs))
	for _, b := range tqbs {
		dto := model.PlanTaskQuestionnaireBindingDTO{
			BindingID:     b.ID.String(),
			TaskID:        b.TaskID.String(),
			TemplateID:    b.TemplateID.String(),
			QuestionOrder: b.QuestionOrder,
			Enabled:       b.Enabled,
		}
		if t, e := r.client.UxQuestionnaireTemplate.Query().Where(uxquestionnairetemplate.ID(b.TemplateID)).Only(ctx); e == nil {
			dto.TemplateName = t.TemplateName
		}
		d.TaskQuestionnaireBindings = append(d.TaskQuestionnaireBindings, dto)
	}

	pqbs, err := r.client.UxPlanProfileQuestionnaireBinding.Query().
		Where(uxplanprofilequestionnairebinding.PlanID(planID)).
		Order(ent.Asc(uxplanprofilequestionnairebinding.FieldQuestionOrder)).
		All(ctx)
	if err != nil {
		return err
	}
	d.ProfileQuestionnaireBindings = make([]model.PlanProfileQuestionnaireBindingDTO, 0, len(pqbs))
	for _, b := range pqbs {
		dto := model.PlanProfileQuestionnaireBindingDTO{
			BindingID:     b.ID.String(),
			ProfileID:     b.ProfileID.String(),
			TemplateID:    b.TemplateID.String(),
			QuestionOrder: b.QuestionOrder,
			Enabled:       b.Enabled,
		}
		if t, e := r.client.UxQuestionnaireTemplate.Query().Where(uxquestionnairetemplate.ID(b.TemplateID)).Only(ctx); e == nil {
			dto.TemplateName = t.TemplateName
		}
		d.ProfileQuestionnaireBindings = append(d.ProfileQuestionnaireBindings, dto)
	}
	return nil
}

// writePlanBindings 把四类绑定 + 模型配置写入数据库
//
// 注意：plan 层的模型配置是"引用项目模型配置"——只持久化 channel + role + 可选超参；
// model_name/api_base_url/api_key_cipher 一律忽略前端输入，model_name 写入占位符
// "<from_project>" 以满足 schema 的 NotEmpty 约束。真正的模型连接信息在启动 run 时
// 从 project.model_config 按 channel 解析后写入 run_model_config。
func writePlanBindings(ctx context.Context, tx *ent.Tx, planID uuid.UUID,
	models []model.PlanModelConfig,
	tbs []model.PlanTaskBindingDTO,
	pbs []model.PlanProfileBindingDTO,
	tqbs []model.PlanTaskQuestionnaireBindingDTO,
	pqbs []model.PlanProfileQuestionnaireBindingDTO,
) error {
	for _, m := range models {
		role := uxplanmodelconfig.ModelRole(m.ModelRole)
		b := tx.UxPlanModelConfig.Create().
			SetPlanID(planID).
			SetModelRole(role).
			SetChannel(m.Channel).
			SetModelName("<from_project>")
		if m.ModelType != "" {
			b = b.SetModelType(uxplanmodelconfig.ModelType(m.ModelType))
		}
		if m.Temperature != nil {
			b = b.SetTemperature(*m.Temperature)
		}
		if m.TopP != nil {
			b = b.SetTopP(*m.TopP)
		}
		if m.MaxTokens != nil {
			b = b.SetMaxTokens(*m.MaxTokens)
		}
		if m.ReasoningEffort != "" {
			b = b.SetReasoningEffort(m.ReasoningEffort)
		}
		if m.ExtraParams != "" {
			b = b.SetExtraParams(m.ExtraParams)
		}
		if _, err := b.Save(ctx); err != nil {
			return err
		}
	}
	for _, t := range tbs {
		tid, err := uuid.Parse(t.TaskID)
		if err != nil {
			continue
		}
		if _, err := tx.UxPlanTaskBinding.Create().
			SetPlanID(planID).
			SetTaskID(tid).
			SetExecutionOrder(t.ExecutionOrder).
			SetEnabled(t.Enabled || !t.Enabled).
			Save(ctx); err != nil {
			return err
		}
	}
	for _, p := range pbs {
		pidu, err := uuid.Parse(p.ProfileID)
		if err != nil {
			continue
		}
		if _, err := tx.UxPlanProfileBinding.Create().
			SetPlanID(planID).
			SetProfileID(pidu).
			SetExecutionOrder(p.ExecutionOrder).
			SetEnabled(p.Enabled || !p.Enabled).
			Save(ctx); err != nil {
			return err
		}
	}
	for _, b := range tqbs {
		tid, err1 := uuid.Parse(b.TaskID)
		tplID, err2 := uuid.Parse(b.TemplateID)
		if err1 != nil || err2 != nil {
			continue
		}
		if _, err := tx.UxPlanTaskQuestionnaireBinding.Create().
			SetPlanID(planID).
			SetTaskID(tid).
			SetTemplateID(tplID).
			SetQuestionOrder(b.QuestionOrder).
			SetEnabled(b.Enabled || !b.Enabled).
			Save(ctx); err != nil {
			return err
		}
	}
	for _, b := range pqbs {
		pidu, err1 := uuid.Parse(b.ProfileID)
		tplID, err2 := uuid.Parse(b.TemplateID)
		if err1 != nil || err2 != nil {
			continue
		}
		if _, err := tx.UxPlanProfileQuestionnaireBinding.Create().
			SetPlanID(planID).
			SetProfileID(pidu).
			SetTemplateID(tplID).
			SetQuestionOrder(b.QuestionOrder).
			SetEnabled(b.Enabled || !b.Enabled).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func entPlanToDetail(p *ent.UxExecutionPlan) *model.PlanDetail {
	d := &model.PlanDetail{
		PlanID:            p.ID.String(),
		ProjectID:         p.ProjectID.String(),
		PlanName:          p.PlanName,
		PlanType:          string(p.PlanType),
		MaxConcurrency:    p.MaxConcurrency,
		StepTimeoutSec:    p.StepTimeoutSec,
		SessionTimeoutSec: p.SessionTimeoutSec,
		RetryLimit:        p.RetryLimit,
		Status:            string(p.Status),
		CreatedBy:         p.CreatedBy.String(),
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
	if p.PromptOverrideID != nil {
		d.PromptOverrideID = p.PromptOverrideID.String()
	}
	if p.Hypothesis != nil {
		d.Hypothesis = *p.Hypothesis
	}
	return d
}

func entPlanModelToDTO(m *ent.UxPlanModelConfig) model.PlanModelConfig {
	dto := model.PlanModelConfig{
		ConfigID:  m.ID.String(),
		ModelRole: string(m.ModelRole),
		Channel:   m.Channel,
		ModelType: string(m.ModelType),
	}
	if m.Temperature != nil {
		v := *m.Temperature
		dto.Temperature = &v
	}
	if m.TopP != nil {
		v := *m.TopP
		dto.TopP = &v
	}
	if m.MaxTokens != nil {
		v := *m.MaxTokens
		dto.MaxTokens = &v
	}
	if m.ReasoningEffort != nil {
		dto.ReasoningEffort = *m.ReasoningEffort
	}
	if m.ExtraParams != nil {
		dto.ExtraParams = *m.ExtraParams
	}
	return dto
}

func maxOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
