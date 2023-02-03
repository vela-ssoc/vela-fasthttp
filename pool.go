package fasthttp

import (
	"os"
	"sort"
	"strings"
	"sync"
)

type poolItem struct {
	id    int
	count uint32
	key   string
	val   PoolItemIFace
}

type PoolItemIFace interface {
	Match(string) bool
	Close() error
	MTime() int64
	Option() interface{}
}

var byteNull = []byte("")

func newPoolItem(key string, val PoolItemIFace) *poolItem {
	return &poolItem{key: key, val: val, count: 0}
}

func (pi *poolItem) Key() string {
	return pi.key
}

func (pi *poolItem) Val() PoolItemIFace {
	return pi.val
}

func (pi *poolItem) Update(val PoolItemIFace) {
	pi.val = val
}

func (pi *poolItem) clear() {
	pi.id = 0
	pi.count = 0
	pi.key = ""
	pi.val = nil
}

type pool struct {
	m sync.RWMutex
	v []*poolItem
}

func newPool() *pool {
	return &pool{
		v: make([]*poolItem, 0),
	}
}

func (p *pool) Len() int {
	return len(p.v)
}

func (p *pool) cap() int {
	return cap(p.v)
}

func (p *pool) Less(i, j int) bool {
	if p.v[i].key == "" {
		return true
	}

	if strings.Compare(p.v[i].key, p.v[j].key) == -1 {
		return true
	}
	return false
}

func (p *pool) Swap(i, j int) {
	//先交换换当前索引
	p.v[i].id, p.v[j].id = j, i

	//在交换对象
	p.v[i], p.v[j] = p.v[j], p.v[i]
}

func (p *pool) GetIdx(idx int) *poolItem {
	if idx < 0 || idx > len(p.v) {
		xEnv.Errorf("invalid pool id %d", idx)
		return nil
	}

	p.m.RLock()
	v := p.v[idx]
	p.m.RUnlock()
	return v
}

func (p *pool) Grep(key string) *poolItem {
	p.m.RLock()
	defer p.m.RUnlock()

	n := p.Len()
	if n == 0 {
		return nil
	}

	for i := 0; i < n; i++ {
		item := p.v[i]
		if item.val.Match(key) {
			return item
		}
	}

	return nil
}

func (p *pool) Get(key string) *poolItem {
	p.m.RLock()
	defer p.m.RUnlock()

	i, j := 0, p.Len()

	var val *poolItem = nil
	for i < j {
		h := int(uint(i+j) >> 1)
		item := p.v[h]
		switch strings.Compare(key, item.key) {
		case 0:
			val = item
			goto done
		case 1:
			i = h + 1
		case -1:
			j = h
		}

	}

done:
	return val
}

func (p *pool) insert(key string, val PoolItemIFace) {
	p.m.Lock()
	defer p.m.Unlock()

	n := p.Len()
	var item *poolItem

	for i := 0; i < n; i++ {
		item = p.v[i]
		//字符串相等
		if strings.EqualFold(item.key, key) {
			item.val = val
			return //覆盖 不需要排序
		}

		if item.key == "" {
			item.key = key
			item.val = val
			goto DONE
		}

	}

	if p.cap() > n {
		p.v = p.v[:n+1]
		item = newPoolItem(key, val)
		item.key = key
		item.val = val
		p.v[n] = item
		goto DONE
	}
	p.v = append(p.v, newPoolItem(key, val))

DONE:
	sort.Sort(p)
}

func (p *pool) reset() {
	p.m.Lock()
	n := p.Len()
	for i := 0; i < n; i++ {
		p.v[i].clear()
	}
	p.v = p.v[:0]
	p.m.Unlock()
}

func (p *pool) clear(prefix string) {
	p.m.Lock()
	n := p.Len()
	k := 0
	for i := 0; i < n; i++ {
		if strings.HasPrefix(p.v[i].key, prefix) {
			xEnv.Errorf("clear %s ... ", p.v[i].key)
			p.v[i].clear()
			k++
		}
	}
	if k != 0 {
		sort.Sort(p)
		p.v = p.v[:n-k]
	}

	p.m.Unlock()
	xEnv.Errorf("%s sync clear succeed", prefix)
}

type compileFn func(string, ...interface{}) (PoolItemIFace, error)

func (p *pool) sync(compile compileFn) {
	p.m.Lock()
	n := p.Len()
	del := 0
	for i := 0; i < n; i++ {
		item := p.v[i]
		if item.key == "" {
			continue
		}

		//判断是否存在
		stat, err := os.Stat(item.key)
		if os.IsNotExist(err) {
			//关闭历史
			if e := item.Val().Close(); e != nil {
				xEnv.Errorf("pool %s close error %v", item.key, e)
			} else {
				xEnv.Errorf("pool %s close succeed", item.key)

			}
			del++
			p.v[i].clear()
			xEnv.Errorf("pool %s delete", item.key)
			continue
		}

		//如果没有修改
		if stat.ModTime().Unix() == item.Val().MTime() {
			continue
		}

		//编译
		if obj, e := compile(item.key, item.val.Option()); e != nil {
			xEnv.Errorf("%s compile error %v", item.key, e)
			continue
		} else {
			if e2 := item.val.Close(); e2 != nil {
				xEnv.Errorf("pool %s close error %v", item.key, e2)
			} else {
				xEnv.Errorf("pool %s close succeed", item.key)
			}
			item.val = obj
			xEnv.Errorf("%s compile succeed", item.key)
		}
	}

	if del != 0 {
		sort.Sort(p)
		p.v = p.v[:n-del]
		xEnv.Errorf("sync delete %d succeed", del)
	}
	p.m.Unlock()
}
