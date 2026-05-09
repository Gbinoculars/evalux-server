package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/uxprojectprompt"

	"github.com/google/uuid"
)

type PromptRepo struct {
	client *ent.Client
}

func NewPromptRepo(client *ent.Client) *PromptRepo {
	return &PromptRepo{client: client}
}

// ListByProjectID 查询项目下所有已自定义的提示词
func (r *PromptRepo) ListByProjectID(ctx context.Context, projectID string) ([]*ent.UxProjectPrompt, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	return r.client.UxProjectPrompt.
		Query().
		Where(uxprojectprompt.ProjectID(pid)).
		All(ctx)
}

// Upsert 保存或更新提示词（若不存在则创建，存在则更新内容）
func (r *PromptRepo) Upsert(ctx context.Context, projectID, promptKey, promptContent string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}

	// 查询是否已存在
	exist, err := r.client.UxProjectPrompt.
		Query().
		Where(
			uxprojectprompt.ProjectID(pid),
			uxprojectprompt.PromptKey(promptKey),
		).
		First(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	if exist != nil {
		// 更新
		return r.client.UxProjectPrompt.
			UpdateOne(exist).
			SetPromptContent(promptContent).
			Exec(ctx)
	}

	// 新建
	return r.client.UxProjectPrompt.
		Create().
		SetProjectID(pid).
		SetPromptKey(promptKey).
		SetPromptContent(promptContent).
		Exec(ctx)
}

// Delete 删除指定提示词（重置为默认时使用）
func (r *PromptRepo) Delete(ctx context.Context, projectID, promptKey string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	_, err = r.client.UxProjectPrompt.
		Delete().
		Where(
			uxprojectprompt.ProjectID(pid),
			uxprojectprompt.PromptKey(promptKey),
		).
		Exec(ctx)
	return err
}
