package safemap

import (
	"sync"
)

type (
	SafeGcMap interface {
		Get(interface{}) (interface{}, bool)
		Set(interface{}, interface{})
		Del(interface{})
		Size() int
	}

	AutoGcMap struct {
		lock       sync.RWMutex
		m          map[interface{}]interface{}
		mBck       map[interface{}]interface{} // 触发阈值, m 与 mBck 调换
		set        map[interface{}]struct{}
		mThreshold int // 阈值
	}
)

func (m *AutoGcMap) Size() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if m.mBck != nil {
		return len(m.m) + len(m.mBck)
	}
	return len(m.m)
}

func (m *AutoGcMap) Get(i interface{}) (v interface{}, exists bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.dataMigration()
	if m.mBck != nil {
		v, exists = m.mBck[i]
		if exists {
			return
		}
	}
	v, exists = m.m[i]
	return
}

func (m *AutoGcMap) Set(i interface{}, v interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.dataMigration()
	m.m[i] = v
	m.set[i] = struct{}{}
	m.swapMap()
}

func (m *AutoGcMap) Del(i interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.dataMigration()
	if m.mBck != nil {
		delete(m.mBck, i)
	}
	delete(m.m, i)
	m.swapMap()
}

// swapMap 交换两个map, 并重置set
func (m *AutoGcMap) swapMap() {
	mSetLength := len(m.set)
	mMLength := len(m.m)
	if mSetLength > m.mThreshold && mMLength <= mSetLength/2 {
		m.set = make(map[interface{}]struct{}, mMLength)
		m.mBck, m.m = m.m, make(map[interface{}]interface{}, m.mThreshold)
	}
	// TODO 启动一个 goroutine 在空闲的时候进行数据迁移
}

// transferData 数据迁移, 每次迁移两个, 均摊到每次操作中
func (m *AutoGcMap) dataMigration() {
	if m.mBck != nil {
		var index int
		for k, v := range m.mBck {
			index++
			if index > 2 {
				break
			}
			m.m[k] = v
			m.set[k] = struct{}{}
			delete(m.mBck, k)
		}
		if len(m.mBck) == 0 {
			m.mBck = nil
		}
	}
}

func NewAutoGcMap(length, mThreshold int) *AutoGcMap {
	return &AutoGcMap{
		m:          make(map[interface{}]interface{}, length),
		mThreshold: mThreshold,
		set:        make(map[interface{}]struct{}, mThreshold),
	}
}
