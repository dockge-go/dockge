package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"

	"dockge/internal/model"
)

// ==================== User ====================

// CreateUser 在单个写事务中只允许首次初始化，避免并发创建多个账号。
func (r *Repository) CreateUser(ctx context.Context, m *model.DockgeUser) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := r.db.Update(func(tx *bbolt.Tx) error {
		// 等待写锁期间也可能取消，取得事务后必须重新检查。
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		if b.Stats().KeyN != 0 {
			return ErrConflict
		}
		id, err := b.NextSequence()
		if err != nil {
			return fmt.Errorf("分配单用户账号 ID：%w", err)
		}
		m.ID = uint(id)
		now := time.Now()
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		if m.UpdatedAt.IsZero() {
			m.UpdatedAt = now
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return b.Put([]byte(m.Username), marshalUser(m))
	})
	return err
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

// updateUser 在 users bucket 中按 ID 定位并以原 key 原地更新。
func (r *Repository) updateUser(ctx context.Context, uid uint, mutate func(*model.DockgeUser)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.db.Update(func(tx *bbolt.Tx) error {
		// 等待写锁期间也可能取消，取得事务后必须重新检查。
		if err := ctx.Err(); err != nil {
			return err
		}
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
		if err := ctx.Err(); err != nil {
			return err
		}
		return b.Put(targetKey, marshalUser(&user))
	})
}

// UpdatePassword 按 ID 定位用户并更新密码哈希。
func (r *Repository) UpdatePassword(ctx context.Context, uid uint, passwordHash string) error {
	return r.updateUser(ctx, uid, func(u *model.DockgeUser) {
		u.Password = passwordHash
	})
}

// SingleUser 只选最早创建的管理员，历史额外账号不再获得登录或会话权限；不静默删除旧数据。
func (r *Repository) SingleUser(ctx context.Context) (model.DockgeUser, error) {
	var result model.DockgeUser
	found := false
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := TxBucket(tx, bucketUsers)
		if err != nil {
			return err
		}
		return b.ForEach(func(_, v []byte) error {
			var u model.DockgeUser
			if err := unmarshalUser(v, &u); err != nil {
				return fmt.Errorf("读取单用户账号：%w", err)
			}
			if u.Role != "" && u.Role != model.RoleAdmin {
				return nil
			}
			if !found || u.ID < result.ID {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.db.Update(func(tx *bbolt.Tx) error {
		// 等待写锁期间也可能取消，取得事务后必须重新检查。
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := TxBucket(tx, bucketSettings)
		if err != nil {
			return err
		}
		payload := settingPayload{Value: value, Type: typ}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
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
