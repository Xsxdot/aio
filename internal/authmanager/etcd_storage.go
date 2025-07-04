package authmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/xsxdot/aio/internal/etcd"

	"github.com/xsxdot/aio/pkg/auth"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// etcd路径前缀
	authPrefixPath       = "/auth"
	userPrefixPath       = authPrefixPath + "/users"            // 用户信息
	credentialPrefixPath = authPrefixPath + "/credentials"      // 用户凭证
	clientCredPath       = authPrefixPath + "/client_creds"     // 客户端凭证
	sessionPrefixPath    = authPrefixPath + "/sessions"         // 会话信息
	rolePrefixPath       = authPrefixPath + "/roles"            // 角色信息
	subjectPrefixPath    = authPrefixPath + "/subjects"         // 主体信息
	subjectsByTypePath   = authPrefixPath + "/subjects_by_type" // 按类型的主体信息
	configPrefixPath     = authPrefixPath + "/config"           // 配置信息
)

// EtcdStorage etcd存储实现
type EtcdStorage struct {
	client *etcd.EtcdClient
}

// NewEtcdStorage 创建etcd存储
func NewEtcdStorage(client *etcd.EtcdClient) *EtcdStorage {
	return &EtcdStorage{
		client: client,
	}
}

// GetUser 获取用户
func (s *EtcdStorage) GetUser(id string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(userPrefixPath, id)
	resp, err := s.client.Client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, ErrUserNotFound
	}

	var user User
	err = json.Unmarshal(resp.Kvs[0].Value, &user)
	if err != nil {
		return nil, fmt.Errorf("unmarshal user failed: %w", err)
	}

	return &user, nil
}

// GetUserByUsername 根据用户名获取用户
func (s *EtcdStorage) GetUserByUsername(username string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 根据用户名查找用户需要遍历
	resp, err := s.client.Client.Get(ctx, userPrefixPath, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	for _, kv := range resp.Kvs {
		var user User
		err := json.Unmarshal(kv.Value, &user)
		if err != nil {
			continue
		}

		if user.Username == username {
			return &user, nil
		}
	}

	return nil, ErrUserNotFound
}

// CreateUser 创建用户
func (s *EtcdStorage) CreateUser(user *User, credential *UserCredential) error {
	// 检查用户名是否已存在
	existing, err := s.GetUserByUsername(user.Username)
	if err == nil && existing != nil {
		return fmt.Errorf("user with username %s already exists", user.Username)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 保存用户信息
	userKey := path.Join(userPrefixPath, user.ID)
	userValue, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshal user failed: %w", err)
	}

	// 保存用户凭证
	credKey := path.Join(credentialPrefixPath, user.Username)
	credValue, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("marshal credential failed: %w", err)
	}

	// 使用事务保证原子性
	_, err = s.client.Client.Txn(ctx).
		Then(
			clientv3.OpPut(userKey, string(userValue)),
			clientv3.OpPut(credKey, string(credValue)),
		).
		Commit()

	if err != nil {
		return fmt.Errorf("save user transaction failed: %w", err)
	}

	return nil
}

// UpdateUser 更新用户
func (s *EtcdStorage) UpdateUser(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 保存用户信息
	userKey := path.Join(userPrefixPath, user.ID)
	userValue, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshal user failed: %w", err)
	}

	_, err = s.client.Client.Put(ctx, userKey, string(userValue))
	if err != nil {
		return fmt.Errorf("update user failed: %w", err)
	}

	return nil
}

// DeleteUser 删除用户
func (s *EtcdStorage) DeleteUser(id string) error {
	// 先获取用户信息
	user, err := s.GetUser(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 删除用户信息
	userKey := path.Join(userPrefixPath, id)
	// 删除用户凭证
	credKey := path.Join(credentialPrefixPath, user.Username)
	// 删除用户主体
	subjectKey := path.Join(subjectPrefixPath, id)

	// 使用事务保证原子性
	_, err = s.client.Client.Txn(ctx).
		Then(
			clientv3.OpDelete(userKey),
			clientv3.OpDelete(credKey),
			clientv3.OpDelete(subjectKey),
		).
		Commit()

	if err != nil {
		return fmt.Errorf("delete user transaction failed: %w", err)
	}

	return nil
}

// ListUsers 列出用户
func (s *EtcdStorage) ListUsers() ([]*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.client.Client.Get(ctx, userPrefixPath, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var users []*User
	for _, kv := range resp.Kvs {
		var user User
		err := json.Unmarshal(kv.Value, &user)
		if err != nil {
			return nil, fmt.Errorf("unmarshal user failed: %w", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// GetUserCredential 获取用户凭证
func (s *EtcdStorage) GetUserCredential(username string) (*UserCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(credentialPrefixPath, username)
	resp, err := s.client.Client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("credential not found for user %s", username)
	}

	var cred UserCredential
	err = json.Unmarshal(resp.Kvs[0].Value, &cred)
	if err != nil {
		return nil, fmt.Errorf("unmarshal credential failed: %w", err)
	}

	return &cred, nil
}

// UpdateUserCredential 更新用户凭证
func (s *EtcdStorage) UpdateUserCredential(username string, credential *UserCredential) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(credentialPrefixPath, username)
	value, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("marshal credential failed: %w", err)
	}

	_, err = s.client.Client.Put(ctx, key, string(value))
	if err != nil {
		return fmt.Errorf("update credential failed: %w", err)
	}

	return nil
}

// SaveRole 保存角色
func (s *EtcdStorage) SaveRole(role *auth.Role) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(rolePrefixPath, role.ID)
	value, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("marshal role failed: %w", err)
	}

	_, err = s.client.Client.Put(ctx, key, string(value))
	if err != nil {
		return fmt.Errorf("save role failed: %w", err)
	}

	return nil
}

// GetRole 获取角色
func (s *EtcdStorage) GetRole(id string) (*auth.Role, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(rolePrefixPath, id)
	resp, err := s.client.Client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("role not found")
	}

	var role auth.Role
	err = json.Unmarshal(resp.Kvs[0].Value, &role)
	if err != nil {
		return nil, fmt.Errorf("unmarshal role failed: %w", err)
	}

	return &role, nil
}

// DeleteRole 删除角色
func (s *EtcdStorage) DeleteRole(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(rolePrefixPath, id)
	_, err := s.client.Client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("delete role failed: %w", err)
	}

	return nil
}

// ListRoles 列出角色
func (s *EtcdStorage) ListRoles() ([]*auth.Role, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.client.Client.Get(ctx, rolePrefixPath, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var roles []*auth.Role
	for _, kv := range resp.Kvs {
		var role auth.Role
		err := json.Unmarshal(kv.Value, &role)
		if err != nil {
			return nil, fmt.Errorf("unmarshal role failed: %w", err)
		}
		roles = append(roles, &role)
	}

	return roles, nil
}

// SaveSubject 保存主体
func (s *EtcdStorage) SaveSubject(subject *auth.Subject) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 保存主体信息
	subjectKey := path.Join(subjectPrefixPath, subject.ID)
	subjectValue, err := json.Marshal(subject)
	if err != nil {
		return fmt.Errorf("marshal subject failed: %w", err)
	}

	// 按类型索引保存主体ID
	typeKey := path.Join(subjectsByTypePath, string(subject.Type), subject.ID)

	// 使用事务保证原子性
	_, err = s.client.Client.Txn(ctx).
		Then(
			clientv3.OpPut(subjectKey, string(subjectValue)),
			clientv3.OpPut(typeKey, ""),
		).
		Commit()

	if err != nil {
		return fmt.Errorf("save subject transaction failed: %w", err)
	}

	return nil
}

// GetSubject 获取主体
func (s *EtcdStorage) GetSubject(id string) (*auth.Subject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(subjectPrefixPath, id)
	resp, err := s.client.Client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("subject not found")
	}

	var subject auth.Subject
	err = json.Unmarshal(resp.Kvs[0].Value, &subject)
	if err != nil {
		return nil, fmt.Errorf("unmarshal subject failed: %w", err)
	}

	return &subject, nil
}

// DeleteSubject 删除主体
func (s *EtcdStorage) DeleteSubject(id string) error {
	// 先获取主体信息，以获取类型
	subject, err := s.GetSubject(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 删除主体信息
	subjectKey := path.Join(subjectPrefixPath, id)
	// 删除按类型索引
	typeKey := path.Join(subjectsByTypePath, string(subject.Type), id)

	// 使用事务保证原子性
	_, err = s.client.Client.Txn(ctx).
		Then(
			clientv3.OpDelete(subjectKey),
			clientv3.OpDelete(typeKey),
		).
		Commit()

	if err != nil {
		return fmt.Errorf("delete subject transaction failed: %w", err)
	}

	return nil
}

// ListSubjects 列出主体
func (s *EtcdStorage) ListSubjects(subjectType auth.SubjectType) ([]*auth.Subject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取指定类型的主体ID列表
	typePrefix := path.Join(subjectsByTypePath, string(subjectType))
	resp, err := s.client.Client.Get(ctx, typePrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return []*auth.Subject{}, nil
	}

	// 根据ID获取主体信息
	var subjects []*auth.Subject
	for _, kv := range resp.Kvs {
		// 从键中提取主体ID
		id := path.Base(string(kv.Key))

		subject, err := s.GetSubject(id)
		if err != nil {
			continue
		}

		subjects = append(subjects, subject)
	}

	return subjects, nil
}

// GetClientCredential 获取客户端凭证
func (s *EtcdStorage) GetClientCredential(clientID string) (*ClientCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(clientCredPath, clientID)
	resp, err := s.client.Client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("client credential not found for client %s", clientID)
	}

	var cred ClientCredential
	err = json.Unmarshal(resp.Kvs[0].Value, &cred)
	if err != nil {
		return nil, fmt.Errorf("unmarshal client credential failed: %w", err)
	}

	return &cred, nil
}

// SaveClientCredential 保存客户端凭证
func (s *EtcdStorage) SaveClientCredential(credential *ClientCredential) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(clientCredPath, credential.ClientID)
	value, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("marshal client credential failed: %w", err)
	}

	_, err = s.client.Client.Put(ctx, key, string(value))
	if err != nil {
		return fmt.Errorf("save client credential failed: %w", err)
	}

	return nil
}

// DeleteClientCredential 删除客户端凭证
func (s *EtcdStorage) DeleteClientCredential(clientID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := path.Join(clientCredPath, clientID)
	_, err := s.client.Client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("delete client credential failed: %w", err)
	}

	return nil
}

// GetConfig 获取配置值
func (s *EtcdStorage) GetConfig(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configKey := path.Join(configPrefixPath, key)
	resp, err := s.client.Client.Get(ctx, configKey)
	if err != nil {
		return "", fmt.Errorf("get config failed: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("config not found: %s", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// SetConfig 设置配置值
func (s *EtcdStorage) SetConfig(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configKey := path.Join(configPrefixPath, key)
	_, err := s.client.Client.Put(ctx, configKey, value)
	if err != nil {
		return fmt.Errorf("set config failed: %w", err)
	}

	return nil
}

// DeleteConfig 删除配置
func (s *EtcdStorage) DeleteConfig(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configKey := path.Join(configPrefixPath, key)
	_, err := s.client.Client.Delete(ctx, configKey)
	if err != nil {
		return fmt.Errorf("delete config failed: %w", err)
	}

	return nil
}
