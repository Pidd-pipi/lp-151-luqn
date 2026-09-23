package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

var (
	ErrIdentityNotFound = errors.New("identity not found")
)

var adjectiveList = []string{"快乐", "沉默", "勇敢", "温柔", "机智", "神秘", "阳光", "清冷"}
var nounList = []string{"树懒", "刺猬", "海豚", "企鹅", "狐狸", "小鹿", "鲸鱼", "松鼠"}

type IdentityService interface {
	Create(nickname, avatar string) (*model.UserIdentity, error)
	GetByKey(key string) (*model.UserIdentity, error)
	GetByID(id uint) (*model.UserIdentity, error)
	GetToken(identity *model.UserIdentity) (string, error)
}

type identityService struct {
	repo    repository.IdentityRepository
	token   TokenService
	logger  *slog.Logger
}

func NewIdentityService(repo repository.IdentityRepository, token TokenService, logger *slog.Logger) IdentityService {
	return &identityService{repo: repo, token: token, logger: logger}
}

func (s *identityService) Create(nickname, avatar string) (*model.UserIdentity, error) {
	if nickname == "" {
		nickname, _ = s.randomNickname()
	}
	key, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate identity key: %w", err)
	}
	if avatar == "" {
		avatar = "https://api.dicebear.com/9.x/bottts-neutral/svg?seed=" + key
	}
	identity := &model.UserIdentity{
		IdentityKey: key,
		Nickname:    nickname,
		Avatar:      avatar,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Create(identity); err != nil {
		return nil, err
	}
	return identity, nil
}

func (s *identityService) GetByKey(key string) (*model.UserIdentity, error) {
	identity, err := s.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrIdentityNotFound
		}
		return nil, err
	}
	return identity, nil
}

func (s *identityService) GetByID(id uint) (*model.UserIdentity, error) {
	identity, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrIdentityNotFound
		}
		return nil, err
	}
	return identity, nil
}

func (s *identityService) GetToken(identity *model.UserIdentity) (string, error) {
	return s.token.Sign(identity.ID, identity.IdentityKey)
}

func (s *identityService) randomNickname() (string, error) {
	ai, err := randInt(len(adjectiveList))
	if err != nil {
		return "", err
	}
	ni, err := randInt(len(nounList))
	if err != nil {
		return "", err
	}
	return adjectiveList[ai] + nounList[ni] + "-" + shortCode(3), nil
}

func generateKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func randInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("rand int: %w", err)
	}
	return int(n.Int64()), nil
}

func shortCode(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[idx.Int64()]
	}
	return string(b)
}
