package repository

import (
	"context"
	"fmt"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/gorm"
)

// CompressionGroupRepository defines the interface for compression group data access
type CompressionGroupRepository interface {
	GetByID(ctx context.Context, id string) (*model.CompressionModelGroup, error)
	GetByProfileAndName(ctx context.Context, profileID, name string) (*model.CompressionModelGroup, error)
	ListByProfile(ctx context.Context, profileID string) ([]model.CompressionModelGroup, error)
	ListEnabledByProfile(ctx context.Context, profileID string) ([]model.CompressionModelGroup, error)
	Create(ctx context.Context, group *model.CompressionModelGroup) error
	Update(ctx context.Context, group *model.CompressionModelGroup) error
	Delete(ctx context.Context, id string) error
}

type compressionGroupRepository struct {
	db *gorm.DB
}

// NewCompressionGroupRepository creates a new compression group repository
func NewCompressionGroupRepository(db *gorm.DB) CompressionGroupRepository {
	return &compressionGroupRepository{db: db}
}

func (r *compressionGroupRepository) GetByID(ctx context.Context, id string) (*model.CompressionModelGroup, error) {
	var group model.CompressionModelGroup
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&group).Error
	if err != nil {
		return nil, fmt.Errorf("compression group not found: %w", err)
	}
	return &group, nil
}

func (r *compressionGroupRepository) GetByProfileAndName(ctx context.Context, profileID, name string) (*model.CompressionModelGroup, error) {
	var group model.CompressionModelGroup
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND name = ?", profileID, name).
		First(&group).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("compression group '%s' not found in profile '%s'", name, profileID)
		}
		return nil, err
	}
	return &group, nil
}

func (r *compressionGroupRepository) ListByProfile(ctx context.Context, profileID string) ([]model.CompressionModelGroup, error) {
	var groups []model.CompressionModelGroup
	err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("priority ASC").
		Find(&groups).Error
	return groups, err
}

func (r *compressionGroupRepository) ListEnabledByProfile(ctx context.Context, profileID string) ([]model.CompressionModelGroup, error) {
	var groups []model.CompressionModelGroup
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND enabled = ?", profileID, true).
		Order("priority ASC").
		Find(&groups).Error
	return groups, err
}

func (r *compressionGroupRepository) Create(ctx context.Context, group *model.CompressionModelGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *compressionGroupRepository) Update(ctx context.Context, group *model.CompressionModelGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *compressionGroupRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.CompressionModelGroup{}, "id = ?", id).Error
}
