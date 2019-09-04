package servicelib

import "time"

// Lend details on books lended to customer
type Lend struct {
	BookID           string
	CustomerID       int
	LatestReturnDate time.Time
}

// Book unique book in library
type Book struct {
	ID          string
	CurrentLend *Lend
	DayPenalty  int
}

// Customer unique customer of library
type Customer struct {
	ID       int
	IsLocked bool
	Age      int
}

// LibraryService the sacred service provided by consultants back in the days
type LibraryService interface {
	GetBook(string) *Book
	GetOldDbBooks() []*Book
	GetCustomer(int) (*Customer, error)
	GetLendsForCustomer(int) ([]*Book, error)
	CollectPayment(int, int) error
	SaveBook(*Book) error
}
