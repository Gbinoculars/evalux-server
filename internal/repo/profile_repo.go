package repo

import (
	"context"
	"encoding/json"

	"evalux-server/ent"
	"evalux-server/ent/uxplanprofilebinding"
	"evalux-server/ent/uxplanprofilequestionnairebinding"
	"evalux-server/ent/uxuserprofile"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type ProfileRepo struct {
	client *ent.Client
}

func NewProfileRepo(client *ent.Client) *ProfileRepo {
	return &ProfileRepo{client: client}
}

func (r *ProfileRepo) Create(ctx context.Context, projectID, profileType, ageGroup, educationLevel, gender string, customFields map[string]interface{}) (*model.ProfileDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	if profileType == "" {
		profileType = "normal"
	}
	b := r.client.UxUserProfile.Create().
		SetProjectID(pid).
		SetProfileType(profileType).
		SetAgeGroup(ageGroup).
		SetEducationLevel(educationLevel).
		SetGender(gender).
		SetEnabled(true)
	if customFields != nil {
		cfJSON, _ := json.Marshal(customFields)
		var cfMap map[string]interface{}
		_ = json.Unmarshal(cfJSON, &cfMap)
		b.SetCustomFields(cfMap)
	}
	p, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entProfileToDetail(p), nil
}

func (r *ProfileRepo) FindByID(ctx context.Context, profileID string) (*model.ProfileDetail, error) {
	pid, err := uuid.Parse(profileID)
	if err != nil {
		return nil, err
	}
	p, err := r.client.UxUserProfile.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	return entProfileToDetail(p), nil
}

func (r *ProfileRepo) Update(ctx context.Context, profileID string, req model.UpdateProfileRequest) (*model.ProfileDetail, error) {
	pid, err := uuid.Parse(profileID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxUserProfile.UpdateOneID(pid)
	if req.AgeGroup != nil {
		upd.SetAgeGroup(*req.AgeGroup)
	}
	if req.EducationLevel != nil {
		upd.SetEducationLevel(*req.EducationLevel)
	}
	if req.Gender != nil {
		upd.SetGender(*req.Gender)
	}
	if req.CustomFields != nil {
		upd.SetCustomFields(req.CustomFields)
	}
	if req.Enabled != nil {
		upd.SetEnabled(*req.Enabled)
	}
	p, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entProfileToDetail(p), nil
}

func (r *ProfileRepo) Delete(ctx context.Context, profileID string) error {
	pid, err := uuid.Parse(profileID)
	if err != nil {
		return err
	}
	// 清理执行计划中该画像的绑定记录
	_, _ = r.client.UxPlanProfileBinding.Delete().
		Where(uxplanprofilebinding.ProfileID(pid)).Exec(ctx)
	_, _ = r.client.UxPlanProfileQuestionnaireBinding.Delete().
		Where(uxplanprofilequestionnairebinding.ProfileID(pid)).Exec(ctx)
	return r.client.UxUserProfile.DeleteOneID(pid).Exec(ctx)
}

func (r *ProfileRepo) ListByProjectID(ctx context.Context, projectID string, query model.ProfileListQuery) ([]model.ProfileDetail, int64, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, 0, err
	}
	q := r.client.UxUserProfile.Query().Where(uxuserprofile.ProjectID(pid))
	if query.Enabled == "true" {
		q = q.Where(uxuserprofile.Enabled(true))
	} else if query.Enabled == "false" {
		q = q.Where(uxuserprofile.Enabled(false))
	}
	if query.ProfileType != "" {
		q = q.Where(uxuserprofile.ProfileType(query.ProfileType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize

	profiles, err := q.Order(ent.Desc(uxuserprofile.FieldCreatedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	list := make([]model.ProfileDetail, 0, len(profiles))
	for _, p := range profiles {
		list = append(list, *entProfileToDetail(p))
	}
	return list, int64(total), nil
}

func entProfileToDetail(p *ent.UxUserProfile) *model.ProfileDetail {
	d := &model.ProfileDetail{
		ProfileID:      p.ID.String(),
		ProjectID:      p.ProjectID.String(),
		ProfileType:    p.ProfileType,
		AgeGroup:       p.AgeGroup,
		EducationLevel: p.EducationLevel,
		Gender:         p.Gender,
		Enabled:        p.Enabled,
		CreatedAt:      p.CreatedAt,
	}
	if p.CustomFields != nil {
		d.CustomFields = p.CustomFields
	}
	return d
}

// BatchDelete 批量删除画像
func (r *ProfileRepo) BatchDelete(ctx context.Context, profileIDs []string) (int, error) {
	uuids := make([]uuid.UUID, 0, len(profileIDs))
	for _, id := range profileIDs {
		uid, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		uuids = append(uuids, uid)
	}
	if len(uuids) == 0 {
		return 0, nil
	}
	// 清理执行计划中这些画像的绑定记录
	_, _ = r.client.UxPlanProfileBinding.Delete().
		Where(uxplanprofilebinding.ProfileIDIn(uuids...)).Exec(ctx)
	_, _ = r.client.UxPlanProfileQuestionnaireBinding.Delete().
		Where(uxplanprofilequestionnairebinding.ProfileIDIn(uuids...)).Exec(ctx)
	deleted, err := r.client.UxUserProfile.Delete().
		Where(uxuserprofile.IDIn(uuids...)).Exec(ctx)
	return deleted, err
}
