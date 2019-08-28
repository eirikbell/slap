package tldr

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/eirikbell/slap/magic"
	"github.com/eirikbell/slap/mocks"
	"github.com/stretchr/testify/assert"
)

func TestFoo(t *testing.T) {
	testCases := []struct {
		houres       float64
		expectedDays int
	}{
		{1, 1},
		{24, 1},
		{0.1, 1},
		{24.00001, 2},
		{48, 2},
		{0, 0},
		{176, 8},
	}
	for _, tt := range testCases {
		result := int(math.Ceil(tt.houres / 24))
		assert.Equal(t, tt.expectedDays, result)
	}
}

func TestShortIdNotFound(t *testing.T) {
	testCases := []struct {
		bookId string
	}{
		{""},
		{"1"},
		{"12"},
		{"123"},
		{"1234"},
	}

	libraryService := new(mocks.LibraryService)
	for _, tt := range testCases {
		err := LendBook(tt.bookId, 123456, libraryService)
		assert.Error(t, err)
		assert.Equal(t, "Book not found", err.Error())
	}
	libraryService.AssertExpectations(t)
}

func TestNotFoundBook(t *testing.T) {
	// testCases := []struct {
	// 	bookId string
	// }{
	// 	{""},
	// 	{"1"},
	// 	{"12"},
	// 	{"123"},
	// 	{"1234"},
	// }

	// libraryService := new(mocks.LibraryService)
	// for _, tt := range testCases {
	// 	err := LendBook(tt.bookId, 123456, libraryService)
	// 	assert.Error(t, err)
	// 	assert.Equal(t, "Book not found", err.Error())
	// }
	// libraryService.AssertExpectations(t)
}

func TestFoundBook(t *testing.T) {
	// testCases := []struct {
	// 	bookId string
	// }{
	// 	{""},
	// 	{"1"},
	// 	{"12"},
	// 	{"123"},
	// 	{"1234"},
	// }

	// libraryService := new(mocks.LibraryService)
	// for _, tt := range testCases {
	// 	err := LendBook(tt.bookId, 123456, libraryService)
	// 	assert.Error(t, err)
	// 	assert.Equal(t, "Book not found", err.Error())
	// }
	// libraryService.AssertExpectations(t)
}

func TestFoundOldDdBook(t *testing.T) {
	// testCases := []struct {
	// 	bookId string
	// }{
	// 	{""},
	// 	{"1"},
	// 	{"12"},
	// 	{"123"},
	// 	{"1234"},
	// }

	// libraryService := new(mocks.LibraryService)
	// for _, tt := range testCases {
	// 	err := LendBook(tt.bookId, 123456, libraryService)
	// 	assert.Error(t, err)
	// 	assert.Equal(t, "Book not found", err.Error())
	// }
	// libraryService.AssertExpectations(t)
}

func TestLendedByOtherCustomer(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	otherCustomerID := 654321

	book := &magic.Book{CurrentLend: &magic.Lend{CustomerID: otherCustomerID}}

	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Book is currently lended to customer %d", otherCustomerID), err.Error())

	libraryService.AssertExpectations(t)
}

func TestCustomerNotFound(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &magic.Book{}

	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(nil, expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Customer not found: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}

func TestCustomerAccountLocked(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	book := &magic.Book{}
	customer := &magic.Customer{IsLocked: true}

	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, "Customer account is locked", err.Error())

	libraryService.AssertExpectations(t)
}

func TestCannotRetrieveCustomerLends(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &magic.Book{}
	customer := &magic.Customer{IsLocked: false}

	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return(nil, expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Cannot retrieve current lends: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}

func TestNotRenewTooManyLends(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	book := &magic.Book{}
	customer := &magic.Customer{IsLocked: false}

	testCases := []struct {
		customerLends []*magic.Book
	}{
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}}},
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}}},
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}}},
	}

	for _, tt := range testCases {
		libraryService := new(mocks.LibraryService)
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return(tt.customerLends, nil)

		err := LendBook(bookID, customerID, libraryService)
		assert.Error(t, err)
		assert.Equal(t, fmt.Sprintf("Customer already has %d lended books, 3 is the limit", len(tt.customerLends)), err.Error())

		libraryService.AssertExpectations(t)
	}
}

func TestRenewTooManyLends(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	book := &magic.Book{CurrentLend: &magic.Lend{CustomerID: customerID}}
	customer := &magic.Customer{IsLocked: false}

	testCases := []struct {
		customerLends []*magic.Book
	}{
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}}},
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}}},
		{[]*magic.Book{&magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}, &magic.Book{}}},
	}

	for _, tt := range testCases {
		libraryService := new(mocks.LibraryService)
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return(tt.customerLends, nil)

		err := LendBook(bookID, customerID, libraryService)
		assert.Error(t, err)
		assert.Equal(t, fmt.Sprintf("Cannot renew when more than 3 other books are lended, customer already has %d lended books", len(tt.customerLends)), err.Error())

		libraryService.AssertExpectations(t)
	}
}

func TestTooYoungToCollectPayment(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	book := &magic.Book{CurrentLend: &magic.Lend{CustomerID: customerID}}

	testCases := []struct {
		age int
	}{
		{0},
		{5},
		{10},
		{11},
		{12},
	}

	for _, tt := range testCases {
		customer := &magic.Customer{IsLocked: false, Age: tt.age}
		libraryService := new(mocks.LibraryService)
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return([]*magic.Book{&magic.Book{CurrentLend: &magic.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}}, nil)

		err := LendBook(bookID, customerID, libraryService)
		assert.Error(t, err)
		assert.Equal(t, fmt.Sprintf("Cannot collect payment for 1 books, customer is younger than 13"), err.Error())

		libraryService.AssertExpectations(t)
	}
}

func TestCannotCollectPayment(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &magic.Book{CurrentLend: &magic.Lend{CustomerID: customerID}}

	customer := &magic.Customer{IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*magic.Book{&magic.Book{DayPenalty: 10, CurrentLend: &magic.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}}, nil)
	libraryService.On("CollectPayment", customerID, 10).Return(expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Payment failed: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}
