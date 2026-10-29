package edit

const killLimit = 20

// killRing 删除内容环形留存;Ctrl+Y 粘贴最近条目,连按向更早轮换
type killRing struct {
	items []string // 旧→新
}

func (k *killRing) push(s string) {
	if s == "" {
		return
	}
	if len(k.items) >= killLimit {
		k.items = k.items[1:]
	}
	k.items = append(k.items, s)
}

// yank 返回最近条目往回数 n 的条目(0=最近),模长回绕;空环 false
func (k *killRing) yank(n int) (string, bool) {
	if len(k.items) == 0 {
		return "", false
	}
	m := len(k.items)
	return k.items[((m-1-n)%m+m)%m], true
}
