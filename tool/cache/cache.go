package cache

type Cache interface {
	Get(string, func() (string, error)) (string, error)
	MGet(...string) ([]interface{}, error)
	Del(string) (bool, error)
	MDel(...string) ([]bool, error)
}
