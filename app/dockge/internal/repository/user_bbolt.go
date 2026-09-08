package repository

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.etcd.io/bbolt"

	"dockge/app/dockge/internal/model"
)

// EncodeUint 将 uint64 编码为 8 字节大端序列（供外部包如 migration 使用）。
func EncodeUint(v uint64) []byte {
	b := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		b[i] = byte(v >> (56 - i*8))
	}
	return b
}

// encodeUint 将 uint 编码为 8 字节大端序列。
func encodeUint(v uint) []byte {
	return EncodeUint(uint64(v))
}

// ==================== User ====================

func (r *Repository) CreateUser(ctx context.Context, m *model.DockgeUser) error {
	err := r.db.Update(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		if b.Get([]byte(m.Username)) != nil {
			return ErrConflict
		}
		id, _ := b.NextSequence()
		m.ID = uint(id)
		return b.Put([]byte(m.Username), marshalUser(m))
	})
	return err
}

// GetUserByUsername 按用户名查询用户记录（users bucket 以 username 为 key）。

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (model.DockgeUser, error) {
	var result model.DockgeUser
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		data := b.Get([]byte(username))
		if data == nil {
			return ErrNotFound
		}
		return unmarshalUser(data, &result)
	})
	return result, err
}

// CountUsers 返回用户总数（用于判断是否需要首次安装引导）。

func (r *Repository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		count = int64(b.Stats().KeyN)
		return nil
	})
	return count, err
}

// GetUser 按 ID 查找用户。users bucket 以 username 为 key（与迁移/创建逻辑一致），
// 因此这里遍历匹配 user.ID，而不是用 encodeUint(uid) 作为 key。
func (r *Repository) GetUser(ctx context.Context, uid uint) (model.DockgeUser, error) {
	var result model.DockgeUser
	var found bool
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		return b.ForEach(func(_, v []byte) error {
			var u model.DockgeUser
			if err := unmarshalUser(v, &u); err != nil {
				return nil
			}
			if u.ID == uid {
				result = u
				found = true
			}
			return nil
		})
	})
	if err == nil && !found {
		return result, ErrNotFound
	}
	return result, err
}

// updateUser 在 users bucket 中按 ID 定位并以原 key 原地更新。
func (r *Repository) updateUser(ctx context.Context, uid uint, mutate func(*model.DockgeUser)) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		var targetKey []byte
		var user model.DockgeUser
		err = b.ForEach(func(k, v []byte) error {
			var u model.DockgeUser
			if err := unmarshalUser(v, &u); err != nil {
				return nil
			}
			if u.ID == uid {
				targetKey = append([]byte{}, k...)
				user = u
			}
			return nil
		})
		if err != nil {
			return err
		}
		if targetKey == nil {
			return ErrNotFound
		}
		mutate(&user)
		user.UpdatedAt = time.Now()
		return b.Put(targetKey, marshalUser(&user))
	})
}

// UpdatePassword 按 ID 定位用户并更新密码哈希。

func (r *Repository) UpdatePassword(ctx context.Context, uid uint, passwordHash string) error {
	return r.updateUser(ctx, uid, func(u *model.DockgeUser) {
		u.Password = passwordHash
	})
}

// UpdateUserTwofa 按 ID 更新 2FA 密钥、防重放令牌与启用状态。

func (r *Repository) UpdateUserTwofa(ctx context.Context, uid uint, secret, lastToken string, status bool) error {
	return r.updateUser(ctx, uid, func(u *model.DockgeUser) {
		u.TwofaSecret = secret
		u.TwofaLastToken = lastToken
		u.TwofaStatus = status
	})
}

// ListUsers 返回全部用户（按 ID 升序）。

func (r *Repository) ListUsers(ctx context.Context) ([]model.DockgeUser, error) {
	var users []model.DockgeUser
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		return b.ForEach(func(_, v []byte) error {
			var u model.DockgeUser
			if err := unmarshalUser(v, &u); err != nil {
				return nil
			}
			users = append(users, u)
			return nil
		})
	})
	// 按 ID 排序
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users, err
}

// ==================== Setting ====================

type settingPayload struct {
	Value string `json:"v"`
	Type  string `json:"t"`
}

// GetSetting 读取单个设置项的值；不存在时返回错误。

func (r *Repository) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketSettings)
		if err != nil {
			return err
		}
		data := b.Get([]byte(key))
		if data == nil {
			return ErrNotFound
		}
		// 兼容旧版纯文本格式和新版 JSON 格式
		var p settingPayload
		if err := json.Unmarshal(data, &p); err == nil {
			value = p.Value
		} else {
			value = string(data)
		}
		return nil
	})
	return value, err
}

// SetSetting 写入设置项（payload 含类型分组）。

func (r *Repository) SetSetting(ctx context.Context, key, value, typ string) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketSettings)
		if err != nil {
			return err
		}
		payload := settingPayload{Value: value, Type: typ}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// GetAllSettingsByType 返回指定分组的全部设置项。

func (r *Repository) GetAllSettingsByType(ctx context.Context, typ string) (map[string]string, error) {
	result := make(map[string]string)
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketSettings)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			var p settingPayload
			if err := json.Unmarshal(v, &p); err != nil {
				return nil
			}
			if p.Type == typ {
				result[string(k)] = p.Value
			}
			return nil
		})
	})
	return result, err
}

// ==================== Helpers ====================

func marshalUser(u *model.DockgeUser) []byte {
	data, _ := json.Marshal(u)
	return data
}

func unmarshalUser(data []byte, u *model.DockgeUser) error {
	return json.Unmarshal(data, u)
}
