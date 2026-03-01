package repository

import (
	"context"
	"fmt"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/gorm"
)

// CompositeModelRepository defines the interface for composite model data access
type CompositeModelRepository interface {
	GetByID(ctx context.Context, id string) (*model.CompositeAutoModel, error)
	GetByProfileAndName(ctx context.Context, profileID, name string) (*model.CompositeAutoModel, error)
	ListByProfile(ctx context.Context, profileID string) ([]model.CompositeAutoModel, error)
	ListEnabledByProfile(ctx context.Context, profileID string) ([]model.CompositeAutoModel, error)
	Create(ctx context.Context, composite *model.CompositeAutoModel) error
	Update(ctx context.Context, composite *model.CompositeAutoModel) error
	Delete(ctx context.Context, id string) error
}

type compositeModelRepository struct {
	db *gorm.DB
}

// NewCompositeModelRepository creates a new composite model repository
func NewCompositeModelRepository(db *gorm.DB) CompositeModelRepository {
	return &compositeModelRepository{db: db}
}

func (r *compositeModelRepository) GetByID(ctx context.Context, id string) (*model.CompositeAutoModel, error) {
	var composite model.CompositeAutoModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&composite).Error
	if err != nil {
		return nil, fmt.Errorf("composite model not found: %w", err)
	}
	return &composite, nil
}

func (r *compositeModelRepository) GetByProfileAndName(ctx context.Context, profileID, name string) (*model.CompositeAutoModel, error) {
	var composite model.CompositeAutoModel
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND name = ?", profileID, name).
		First(&composite).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("composite model '%s' not found in profile '%s'", name, profileID)
		}
		return nil, err
	}
	return &composite, nil
}

func (r *compositeModelRepository) ListByProfile(ctx context.Context, profileID string) ([]model.CompositeAutoModel, error) {
	var composites []model.CompositeAutoModel
	err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("priority ASC").
		Find(&composites).Error
	return composites, err
}

func (r *compositeModelRepository) ListEnabledByProfile(ctx context.Context, profileID string) ([]model.CompositeAutoModel, error) {
	var composites []model.CompositeAutoModel
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND enabled = ?", profileID, true).
		Order("priority ASC").
		Find(&composites).Error
	return composites, err
}

func (r *compositeModelRepository) Create(ctx context.Context, composite *model.CompositeAutoModel) error {
	return r.db.WithContext(ctx).Create(composite).Error
}

func (r *compositeModelRepository) Update(ctx context.Context, composite *model.CompositeAutoModel) error {
	return r.db.WithContext(ctx).Save(composite).Error
}

func (r *compositeModelRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.CompositeAutoModel{}, "id = ?", id).Error
}
