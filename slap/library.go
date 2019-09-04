package tldr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eirikbell/slap/servicelib"
	"github.com/pkg/errors"
)

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

func isRenewal(book *servicelib.Book, customerID int) (bool, error) {
	if book.CurrentLend != nil {
		if book.CurrentLend.CustomerID != customerID {
			return false, fmt.Errorf("Book is currently lended to customer %d", book.CurrentLend.CustomerID)
		}

		return true, nil
	}
	return false, nil
}

func findBookDetails(bookID string, customerID int, libraryService servicelib.LibraryService) (*servicelib.Book, bool, error) {
	book, err := findBook(bookID, libraryService)
	if err != nil {
		return nil, false, err
	}

	renewal, err := isRenewal(book, customerID)
	if err != nil {
		return nil, false, err
	}

	return book, renewal, nil
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

func handleReturns(customer *servicelib.Customer, nonreturned []*servicelib.Book, libraryService servicelib.LibraryService) error {
	if len(nonreturned) > 0 {
		// Not allowed by law to collect payment if customer is younger than 13
		if customer.Age < 13 {
			return fmt.Errorf("Cannot collect payment for %d books, customer is younger than 13", len(nonreturned))
		}

		tot := 0
		for _, nr := range nonreturned {
			late := time.Since(nr.CurrentLend.LatestReturnDate)
			days := int(math.Ceil(late.Hours() / 24))

			price := days * nr.DayPenalty
			tot += price
		}
		if tot > 0 {
			if customer.Age < 18 {
				// 50% less if not adult
				tot = int(math.Ceil(float64(tot) / float64(2)))
			}
			err := libraryService.CollectPayment(customer.ID, tot)
			if err != nil {
				return errors.Wrap(err, "Payment failed")
			}

			fail := []string{}
			for _, nr := range nonreturned {
				d := time.Now().AddDate(0, 0, 7)
				nr.CurrentLend.LatestReturnDate = d
				// Must manually register later
				if err := libraryService.SaveBook(nr); err != nil {
					fail = append(fail, nr.ID)
				}
			}
			if len(fail) > 0 {
				return fmt.Errorf("Saving extended date failed, manually register extension for customer %d on books %s", customer.ID, strings.Join(fail, ", "))
			}
		}
	}

	return nil
}

// LendBook handles the transaction of lending a book to a customer
func LendBook(bookID string, customerID int, lib servicelib.LibraryService) error {
	b, renewal, err := findBookDetails(bookID, customerID, lib)
	if err != nil {
		return err
	}

	c, err := findActiveCustomer(customerID, lib)
	if err != nil {
		return err
	}

	// Prerequisites for lend
	cl, err := lib.GetLendsForCustomer(customerID)
	if err != nil {
		return errors.Wrap(err, "Cannot retrieve current lends")
	}
	// Used to be more
	if len(cl) >= 3 {
		if !renewal {
			return fmt.Errorf("Customer already has %d lended books, 3 is the limit", len(cl))
		}

		// Trying to bring down outstanding books, but allow renewal
		if len(cl) >= 4 {
			return fmt.Errorf("Cannot renew when more than 3 other books are lended, customer already has %d lended books", len(cl))
		}
	}

	nonreturned := []*servicelib.Book{}
	for _, l := range cl {
		if l.CurrentLend.LatestReturnDate.Before(time.Now()) {
			nonreturned = append(nonreturned, l)
		}
	}

	// Collect payment for missing return
	if err := handleReturns(c, nonreturned, lib); err != nil {
		return err
	}
	// if len(nonreturned) > 0 {
	// 	// Not allowed by law to collect payment if customer is younger than 13
	// 	if c.Age < 13 {
	// 		return fmt.Errorf("Cannot collect payment for %d books, customer is younger than 13", len(nonreturned))
	// 	}

	// 	tot := 0
	// 	for _, nr := range nonreturned {
	// 		late := time.Since(nr.CurrentLend.LatestReturnDate)
	// 		days := int(math.Ceil(late.Hours() / 24))

	// 		price := days * nr.DayPenalty
	// 		tot += price
	// 	}
	// 	if tot > 0 {
	// 		if c.Age < 18 {
	// 			// 50% less if not adult
	// 			tot = int(math.Ceil(float64(tot) / float64(2)))
	// 		}
	// 		err := lib.CollectPayment(customerID, tot)
	// 		if err != nil {
	// 			return errors.Wrap(err, "Payment failed")
	// 		}

	// 		fail := []string{}
	// 		for _, nr := range nonreturned {
	// 			d := time.Now().AddDate(0, 0, 7)
	// 			nr.CurrentLend.LatestReturnDate = d
	// 			// Must manually register later
	// 			if err := lib.SaveBook(nr); err != nil {
	// 				fail = append(fail, nr.ID)
	// 			}
	// 		}
	// 		if len(fail) > 0 {
	// 			return fmt.Errorf("Saving extended date failed, manually register extension for customer %d on books %s", customerID, strings.Join(fail, ", "))
	// 		}
	// 	}
	// }

	// Create book lending
	if renewal {
		d := time.Now().AddDate(0, 0, 7)
		b.CurrentLend.LatestReturnDate = d
		// Must manually refund
		if err := lib.SaveBook(b); err != nil {
			return errors.Wrap(err, "Renewal failed")
		}
	} else {
		d := time.Now().AddDate(0, 0, 7)
		b.CurrentLend = &servicelib.Lend{
			CustomerID:       customerID,
			BookID:           bookID,
			LatestReturnDate: d,
		}
		// Lend registration failed
		if err := lib.SaveBook(b); err != nil {
			return errors.Wrap(err, "Lend failed")
		}
	}

	return nil
}
