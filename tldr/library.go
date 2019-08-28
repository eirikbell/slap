package tldr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eirikbell/slap/magic"
	"github.com/pkg/errors"
)

// LendBook handles the transaction of lending a book to a customer
func LendBook(bID string, cID int, lib magic.LibraryService) error {
	bidlen := len(bID)

	var b *magic.Book
	// Check book is lendable
	if bidlen < 5 {
		b = nil
	} else {
		b = lib.GetBook(bID)
		if b == nil {
			olddb := lib.GetOldDbBooks()
			for _, ob := range olddb {
				if ob.ID == bID {
					b = ob
					break
				}
			}
		}
	}
	if b == nil {
		return fmt.Errorf("Book not found")
	}

	renewal := false
	if b.CurrentLend != nil {
		if b.CurrentLend.CustomerID != cID {
			return fmt.Errorf("Book is currently lended to customer %d", b.CurrentLend.CustomerID)
		}

		renewal = true
	}

	// Check customer has no outstanding returns
	c, err := lib.GetCustomer(cID)
	if err != nil {
		return errors.Wrap(err, "Customer not found")
	}

	if c.IsLocked {
		return fmt.Errorf("Customer account is locked")
	}

	cl, err := lib.GetLendsForCustomer(cID)
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

	nonreturned := []*magic.Book{}
	for _, l := range cl {
		if l.CurrentLend.LatestReturnDate.Before(time.Now()) {
			nonreturned = append(nonreturned, l)
		}
	}

	// Collect payment for missing return
	if len(nonreturned) > 0 {
		// Not allowed by law to collect payment if customer is younger than 13
		if c.Age < 13 {
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
			if c.Age < 18 {
				// 50% less if not adult
				tot = tot / 2
			}
			err := lib.CollectPayment(cID, tot)
			if err != nil {
				return errors.Wrap(err, "Payment failed")
			}

			fail := []string{}
			for _, nr := range nonreturned {
				d := time.Now().AddDate(0, 0, 7)
				nr.CurrentLend.LatestReturnDate = d
				// Must manually register later
				if err := lib.SaveBook(nr); err != nil {
					fail = append(fail, nr.ID)
				}
			}
			if len(fail) > 0 {
				return fmt.Errorf("Saving extended date failed, manually register extension for customer %d on books %s", cID, strings.Join(fail, ", "))
			}
		}
	}

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
		b.CurrentLend = &magic.Lend{
			CustomerID:       cID,
			BookID:           bID,
			LatestReturnDate: d,
		}
		// Must manually refund
		if err := lib.SaveBook(b); err != nil {
			return errors.Wrap(err, "Renewal failed")
		}
	}

	return nil
}
