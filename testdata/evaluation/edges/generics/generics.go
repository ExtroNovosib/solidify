package generics

// Repository verifies that interface-width accounting includes a type
// parameter without treating the type parameter as a method.
type Repository[T any] interface {
	Create(T)
	Read(T)
	Update(T)
	Delete(T)
	List() []T
	Count() int
	Exists(T) bool
	Replace(T)
	Archive(T)
}
