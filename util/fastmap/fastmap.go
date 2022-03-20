package fastmap

import (
	"cbsignal/client"
)

type FastMap interface {
     CountNoLock() int
	 CountPerMapNoLock() []int
	 Set(key string, value *client.Client)
	 Get(key string) (*client.Client, bool)
	 Has(key string) bool
	 Remove(key string)
	 Clear()
	 Range(f func(key string, value *client.Client) bool)

}


