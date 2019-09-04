package tldr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eirikbell/slap/servicelib"
	"github.com/pkg/errors"
)

// LendBook handles the transaction of lending a book to a customer
func LendBook(bookID string, customerID int, libraryService servicelib.LibraryService) error {
	book, isRenewal, err := findBookDetails(bookID, customerID, libraryService)
	if err != nil {
		return err
	}

	customer, err := findActiveCustomer(customerID, libraryService)
	if err != nil {
		return err
	}

	err = handleReturns(customer, isRenewal, libraryService)
	if err != nil {
		return err
	}

	return lendOrRenewBook(customer, book, isRenewal, libraryService)
}

func findBookDetails(bookID string, customerID int, libraryService servicelib.LibraryService) (*servicelib.Book, bool, error) {
	book, err := findBook(bookID, libraryService)
	if err != nil {
		return nil, false, err
	}

	isRenewal, err := isisRenewal(book, customerID)
	if err != nil {
		return nil, false, err
	}

	return book, isRenewal, nil
}

func findBook(bookID string, libraryService servicelib.LibraryService) (*servicelib.Book, error) {
	var b *servicelib.Book
	// Check book is lendable
	if len(bookID) < 5 {
		return nil, fmt.Errorf("Book not found")
	}

	b = libraryService.GetBook(bookID)
	if b != nil {
		return b, nil
	}

	olddb := libraryService.GetOldDbBooks()
	for _, ob := range olddb {
		if ob.ID == bookID {
			return ob, nil
		}
	}

	return nil, fmt.Errorf("Book not found")
}

func isisRenewal(book *servicelib.Book, customerID int) (bool, error) {
	if book.CurrentLend != nil {
		if book.CurrentLend.CustomerID != customerID {
			return false, fmt.Errorf("Book is currently lended to customer %d", book.CurrentLend.CustomerID)
		}

		return true, nil
	}
	return false, nil
}

func handleReturns(customer *servicelib.Customer, isRenewal bool, libraryService servicelib.LibraryService) error {
	notReturnedBookLends, err := getNotReturnedBookLends(customer, isRenewal, libraryService)
	if err != nil {
		return err
	}

	return collectPayment(customer, notReturnedBookLends, libraryService)
}

func findActiveCustomer(customerID int, libraryService servicelib.LibraryService) (*servicelib.Customer, error) {
	customer, err := libraryService.GetCustomer(customerID)
	if err != nil {
		return nil, errors.Wrap(err, "Customer not found")
	}

	if customer.IsLocked {
		return nil, fmt.Errorf("Customer account is locked")
	}

	return customer, nil
}

func getNotReturnedBookLends(customer *servicelib.Customer, isRenewal bool, libraryService servicelib.LibraryService) ([]*servicelib.Book, error) {
	bookLends, err := libraryService.GetLendsForCustomer(customer.ID)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot retrieve current lends")
	}

	if err := validateLendingLimitNotExceeded(bookLends, isRenewal); err != nil {
		return nil, err
	}

	return filterNotReturnedBookLends(bookLends), nil
}

func validateLendingLimitNotExceeded(bookLends []*servicelib.Book, isRenewal bool) error {
	// Used to be more
	if len(bookLends) >= 3 {
		if !isRenewal {
			return fmt.Errorf("Customer already has %d lended books, 3 is the limit", len(bookLends))
		}

		// Trying to bring down outstanding books, but allow renewal if 3 other outstanding books
		if len(bookLends) >= 4 {
			return fmt.Errorf("Cannot renew when more than 3 other books are lended, customer already has %d lended books", len(bookLends))
		}
	}
	return nil
}

func filterNotReturnedBookLends(bookLends []*servicelib.Book) []*servicelib.Book {
	notReturnedBookLends := []*servicelib.Book{}
	for _, l := range bookLends {
		if l.CurrentLend.LatestReturnDate.Before(time.Now()) {
			notReturnedBookLends = append(notReturnedBookLends, l)
		}
	}
	return notReturnedBookLends
}

func collectPayment(customer *servicelib.Customer, notReturnedBookLends []*servicelib.Book, libraryService servicelib.LibraryService) error {
	if len(notReturnedBookLends) == 0 {
		return nil
	}

	if err := canCollectPayment(customer, notReturnedBookLends); err != nil {
		return err
	}

	return payAndRenewBookLends(customer, notReturnedBookLends, libraryService)
}

func canCollectPayment(customer *servicelib.Customer, bookLends []*servicelib.Book) error {
	// Not allowed by law to collect payment if customer is younger than 13
	if customer.Age < 13 {
		return fmt.Errorf("Cannot collect payment for %d books, customer is younger than 13", len(bookLends))
	}
	return nil
}

func payAndRenewBookLends(customer *servicelib.Customer, bookLends []*servicelib.Book, libraryService servicelib.LibraryService) error {
	priceToPay := calculateTotalPriceForLateReturn(customer, bookLends)

	if priceToPay > 0 {
		if err := libraryService.CollectPayment(customer.ID, priceToPay); err != nil {
			return errors.Wrap(err, "Payment failed")
		}

		if err := renewBookLends(customer, bookLends, libraryService); err != nil {
			return err
		}
	}
	return nil
}

func calculateTotalPriceForLateReturn(customer *servicelib.Customer, bookLends []*servicelib.Book) int {
	tot := 0
	for _, nr := range bookLends {
		price := calculatePriceForLateReturn(nr)
		tot += price
	}
	if customer.Age < 18 {
		// 50% less if not adult
		tot = int(math.Ceil(float64(tot) / float64(2)))
	}
	return tot
}

func calculatePriceForLateReturn(book *servicelib.Book) int {
	late := time.Since(book.CurrentLend.LatestReturnDate)
	days := int(math.Ceil(late.Hours() / 24))

	return days * book.DayPenalty
}

func renewBookLends(customer *servicelib.Customer, bookLends []*servicelib.Book, libraryService servicelib.LibraryService) error {
	fail := []string{}
	for _, book := range bookLends {
		setBookLendLatestReturnDate(book.CurrentLend)
		// Must manually register later
		if err := libraryService.SaveBook(book); err != nil {
			fail = append(fail, book.ID)
		}
	}
	if len(fail) > 0 {
		return fmt.Errorf("Saving extended date failed, manually register extension for customer %d on books %s", customer.ID, strings.Join(fail, ", "))
	}
	return nil
}

func lendOrRenewBook(customer *servicelib.Customer, book *servicelib.Book, isRenewal bool, libraryService servicelib.LibraryService) error {
	if isRenewal {
		return renewBook(book, libraryService)
	}

	return lendBook(book, customer.ID, libraryService)
}

func lendBook(book *servicelib.Book, customerID int, libraryService servicelib.LibraryService) error {
	book.CurrentLend = createBookLend(customerID, book.ID)
	// Lend registration failed
	if err := libraryService.SaveBook(book); err != nil {
		return errors.Wrap(err, "Lend failed")
	}

	return nil
}

func renewBook(book *servicelib.Book, libraryService servicelib.LibraryService) error {
	setBookLendLatestReturnDate(book.CurrentLend)
	// Must manually refund
	if err := libraryService.SaveBook(book); err != nil {
		return errors.Wrap(err, "Renewal failed")
	}
	return nil
}

func createBookLend(customerID int, bookID string) *servicelib.Lend {
	lend := &servicelib.Lend{
		CustomerID: customerID,
		BookID:     bookID,
	}
	setBookLendLatestReturnDate(lend)
	return lend
}

func setBookLendLatestReturnDate(lend *servicelib.Lend) {
	d := time.Now().AddDate(0, 0, 7)
	lend.LatestReturnDate = d
}
