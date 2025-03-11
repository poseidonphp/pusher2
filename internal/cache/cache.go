package cache

type Contract interface {
	Get(key string) (string, bool)
	Set(key string, value string) // create or update the key with the value
	Delete(key string)
	Remember(key string, ttl int, callback func() (string, error)) (string, error)
	Has(key string) bool
	Update(key string, value string) // Update the value only if it exists
}
