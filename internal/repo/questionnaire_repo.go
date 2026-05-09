package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/uxquestionnairequestion"
	"evalux-server/ent/uxquestionnairetemplate"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type QuestionnaireRepo struct {
	client *ent.Client
}

func NewQuestionnaireRepo(client *ent.Client) *QuestionnaireRepo {
	return &QuestionnaireRepo{client: client}
}

func (r *QuestionnaireRepo) CreateTemplate(ctx context.Context, req model.CreateQuestionnaireRequest) (*model.QuestionnaireDetail, error) {
	b := r.client.UxQuestionnaireTemplate.Create().
		SetTemplateName(req.TemplateName).
		SetDimensionSchema(req.DimensionSchema).
		SetStatus("ACTIVE")
	if req.ProjectID != "" {
		pid, err := uuid.Parse(req.ProjectID)
		if err == nil {
			b.SetProjectID(pid)
		}
	}
	if req.TemplateDesc != "" {
		b.SetTemplateDesc(req.TemplateDesc)
	}
	t, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entTemplateToDetail(t), nil
}

func (r *QuestionnaireRepo) FindTemplateByID(ctx context.Context, templateID string) (*model.QuestionnaireDetail, error) {
	tid, err := uuid.Parse(templateID)
	if err != nil {
		return nil, err
	}
	t, err := r.client.UxQuestionnaireTemplate.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	return entTemplateToDetail(t), nil
}

func (r *QuestionnaireRepo) UpdateTemplate(ctx context.Context, templateID string, req model.UpdateQuestionnaireRequest) (*model.QuestionnaireDetail, error) {
	tid, err := uuid.Parse(templateID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxQuestionnaireTemplate.UpdateOneID(tid)
	if req.TemplateName != nil {
		upd.SetTemplateName(*req.TemplateName)
	}
	if req.DimensionSchema != nil {
		upd.SetDimensionSchema(req.DimensionSchema)
	}
	if req.TemplateDesc != nil {
		upd.SetTemplateDesc(*req.TemplateDesc)
	}
	if req.Status != nil {
		upd.SetStatus(uxquestionnairetemplate.Status(*req.Status))
	}
	t, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entTemplateToDetail(t), nil
}

func (r *QuestionnaireRepo) DeleteTemplate(ctx context.Context, templateID string) error {
	tid, err := uuid.Parse(templateID)
	if err != nil {
		return err
	}
	// 先删除该模板下的所有题目（避免外键约束）
	if _, err := r.client.UxQuestionnaireQuestion.Delete().
		Where(uxquestionnairequestion.TemplateID(tid)).Exec(ctx); err != nil {
		return err
	}
	return r.client.UxQuestionnaireTemplate.DeleteOneID(tid).Exec(ctx)
}

func (r *QuestionnaireRepo) ListTemplatesByProjectID(ctx context.Context, projectID string, query model.QuestionnaireListQuery) ([]model.QuestionnaireDetail, int64, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, 0, err
	}
	q := r.client.UxQuestionnaireTemplate.Query().Where(uxquestionnairetemplate.ProjectID(pid))
	if query.Status != "" {
		q = q.Where(uxquestionnairetemplate.StatusEQ(uxquestionnairetemplate.Status(query.Status)))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize
	templates, err := q.Order(ent.Desc(uxquestionnairetemplate.FieldCreatedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	list := make([]model.QuestionnaireDetail, 0, len(templates))
	for _, t := range templates {
		list = append(list, *entTemplateToDetail(t))
	}
	return list, int64(total), nil
}

// CreateQuestion 创建问卷题目
func (r *QuestionnaireRepo) CreateQuestion(ctx context.Context, req model.CreateQuestionRequest) (*model.QuestionDetail, error) {
	tid, err := uuid.Parse(req.TemplateID)
	if err != nil {
		return nil, err
	}
	b := r.client.UxQuestionnaireQuestion.Create().
		SetTemplateID(tid).
		SetQuestionNo(req.QuestionNo).
		SetQuestionType(uxquestionnairequestion.QuestionType(req.QuestionType)).
		SetQuestionText(req.QuestionText).
		SetIsRequired(req.IsRequired)
	if req.OptionList != nil {
		b.SetOptionList(req.OptionList)
	}
	if req.ScoreRange != nil {
		b.SetScoreRange(req.ScoreRange)
	}
	if req.DimensionCode != "" {
		b.SetDimensionCode(req.DimensionCode)
	}
	q, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entQuestionToDetail(q), nil
}

// ListQuestionsByTemplateID 查询问卷题目列表
func (r *QuestionnaireRepo) ListQuestionsByTemplateID(ctx context.Context, templateID string) ([]model.QuestionDetail, error) {
	tid, err := uuid.Parse(templateID)
	if err != nil {
		return nil, err
	}
	questions, err := r.client.UxQuestionnaireQuestion.Query().
		Where(uxquestionnairequestion.TemplateID(tid)).
		Order(ent.Asc(uxquestionnairequestion.FieldQuestionNo)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.QuestionDetail, 0, len(questions))
	for _, q := range questions {
		list = append(list, *entQuestionToDetail(q))
	}
	return list, nil
}

// DeleteQuestion 删除题目
func (r *QuestionnaireRepo) DeleteQuestion(ctx context.Context, questionID string) error {
	qid, err := uuid.Parse(questionID)
	if err != nil {
		return err
	}
	return r.client.UxQuestionnaireQuestion.DeleteOneID(qid).Exec(ctx)
}

// UpdateQuestion 更新题目
func (r *QuestionnaireRepo) UpdateQuestion(ctx context.Context, questionID string, req model.UpdateQuestionRequest) (*model.QuestionDetail, error) {
	qid, err := uuid.Parse(questionID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxQuestionnaireQuestion.UpdateOneID(qid)
	if req.QuestionNo != nil {
		upd.SetQuestionNo(*req.QuestionNo)
	}
	if req.QuestionType != nil {
		upd.SetQuestionType(uxquestionnairequestion.QuestionType(*req.QuestionType))
	}
	if req.QuestionText != nil {
		upd.SetQuestionText(*req.QuestionText)
	}
	if req.OptionList != nil {
		upd.SetOptionList(req.OptionList)
	}
	if req.ScoreRange != nil {
		upd.SetScoreRange(req.ScoreRange)
	}
	if req.DimensionCode != nil {
		if *req.DimensionCode == "" {
			upd.ClearDimensionCode()
		} else {
			upd.SetDimensionCode(*req.DimensionCode)
		}
	}
	if req.IsRequired != nil {
		upd.SetIsRequired(*req.IsRequired)
	}
	q, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entQuestionToDetail(q), nil
}

// ReorderQuestions 重排题目顺序
func (r *QuestionnaireRepo) ReorderQuestions(ctx context.Context, templateID string, questionIDs []string) error {
	for i, qidStr := range questionIDs {
		qid, err := uuid.Parse(qidStr)
		if err != nil {
			return err
		}
		if err := r.client.UxQuestionnaireQuestion.UpdateOneID(qid).
			SetQuestionNo(i + 1).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func entTemplateToDetail(t *ent.UxQuestionnaireTemplate) *model.QuestionnaireDetail {
	d := &model.QuestionnaireDetail{
		TemplateID:   t.ID.String(),
		TemplateName: t.TemplateName,
		Status:       string(t.Status),
		CreatedAt:    t.CreatedAt,
	}
	if t.ProjectID != nil {
		d.ProjectID = t.ProjectID.String()
	}
	if t.DimensionSchema != nil {
		d.DimensionSchema = t.DimensionSchema
	}
	if t.TemplateDesc != nil {
		d.TemplateDesc = *t.TemplateDesc
	}
	return d
}

func entQuestionToDetail(q *ent.UxQuestionnaireQuestion) *model.QuestionDetail {
	d := &model.QuestionDetail{
		QuestionID:   q.ID.String(),
		TemplateID:   q.TemplateID.String(),
		QuestionNo:   q.QuestionNo,
		QuestionType: string(q.QuestionType),
		QuestionText: q.QuestionText,
		IsRequired:   q.IsRequired,
	}
	if q.OptionList != nil {
		d.OptionList = q.OptionList
	}
	if q.ScoreRange != nil {
		d.ScoreRange = q.ScoreRange
	}
	if q.DimensionCode != nil {
		d.DimensionCode = *q.DimensionCode
	}
	return d
}
