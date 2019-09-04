package tldr

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/eirikbell/slap/mocks"
	"github.com/eirikbell/slap/servicelib"
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
		bookID string
	}{
		{""},
		{"1"},
		{"12"},
		{"123"},
		{"1234"},
	}

	for _, tt := range testCases {
		libraryService := new(mocks.LibraryService)

		err := LendBook(tt.bookID, 123456, libraryService)
		assert.Error(t, err)
		assert.Equal(t, "Book not found", err.Error())

		libraryService.AssertExpectations(t)
	}
}

func TestBookNotFound(t *testing.T) {
	testCases := []struct {
		bookID string
	}{
		{"12345"},
		{"23456"},
	}

	for _, tt := range testCases {
		libraryService := new(mocks.LibraryService)
		libraryService.On("GetBook", tt.bookID).Return(nil)
		libraryService.On("GetOldDbBooks").Return([]*servicelib.Book{&servicelib.Book{ID: "54321"}, &servicelib.Book{ID: "65432"}})

		err := LendBook(tt.bookID, 123456, libraryService)
		assert.Error(t, err)
		assert.Equal(t, "Book not found", err.Error())

		libraryService.AssertExpectations(t)
	}
}

func TestLendedByOtherCustomer(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	otherCustomerID := 654321

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: otherCustomerID}}

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

	book := &servicelib.Book{}

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

	book := &servicelib.Book{}
	customer := &servicelib.Customer{ID: customerID, IsLocked: true}

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

	book := &servicelib.Book{}
	customer := &servicelib.Customer{ID: customerID, IsLocked: false}

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

	book := &servicelib.Book{}
	customer := &servicelib.Customer{ID: customerID, IsLocked: false}

	testCases := []struct {
		customerLends []*servicelib.Book
	}{
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
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

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID}}
	customer := &servicelib.Customer{ID: customerID, IsLocked: false}

	testCases := []struct {
		customerLends []*servicelib.Book
	}{
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
		{[]*servicelib.Book{&servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}, &servicelib.Book{}}},
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

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID}}

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
		customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: tt.age}
		libraryService := new(mocks.LibraryService)
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{&servicelib.Book{CurrentLend: &servicelib.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}}, nil)

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

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{&servicelib.Book{DayPenalty: 10, CurrentLend: &servicelib.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}}, nil)
	libraryService.On("CollectPayment", customerID, 10).Return(expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Payment failed: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}

func TestFailingExtendedLend(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID}}

	nonReturnedBook1 := &servicelib.Book{ID: "id1", DayPenalty: 10, CurrentLend: &servicelib.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}
	nonReturnedBook2 := &servicelib.Book{ID: "id2", DayPenalty: 10, CurrentLend: &servicelib.Lend{LatestReturnDate: time.Now().Add(-1 * time.Minute)}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{nonReturnedBook1, nonReturnedBook2}, nil)
	libraryService.On("CollectPayment", customerID, 20).Return(nil)
	libraryService.On("SaveBook", nonReturnedBook1).Return(expectedErr)
	libraryService.On("SaveBook", nonReturnedBook2).Return(expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Saving extended date failed, manually register extension for customer %d on books %s, %s", customerID, nonReturnedBook1.ID, nonReturnedBook2.ID), err.Error())

	libraryService.AssertExpectations(t)
}

func TestRenewalFails(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().AddDate(0, 0, 1)}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book}, nil)
	libraryService.On("SaveBook", book).Return(expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Renewal failed: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}

func TestRenewalSucceeds(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: oldDate}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book}, nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestRenewalSucceedsWithBookInOldDb(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: oldDate}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(nil)
	libraryService.On("GetOldDbBooks").Return([]*servicelib.Book{book})
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book}, nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestRenewalSucceedsCollectPayment(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: oldDate}}
	nonReturnedBook := &servicelib.Book{ID: "654321", DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(-1 * time.Minute)}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book, nonReturnedBook}, nil)
	libraryService.On("CollectPayment", customerID, nonReturnedBook.DayPenalty).Return(nil)
	libraryService.On("SaveBook", nonReturnedBook).Return(nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestLendFails(t *testing.T) {
	bookID := "12345"
	customerID := 123456
	expectedErr := fmt.Errorf("DB error")

	book := &servicelib.Book{ID: bookID, DayPenalty: 10}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{}, nil)
	libraryService.On("SaveBook", book).Return(expectedErr)

	err := LendBook(bookID, customerID, libraryService)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Lend failed: %s", expectedErr.Error()), err.Error())

	libraryService.AssertExpectations(t)
}

func TestLendSucceeds(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{}, nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestLendSucceedsWithBookInOldDb(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(nil)
	libraryService.On("GetOldDbBooks").Return([]*servicelib.Book{book})
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{}, nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestLendSucceedsCollectPayment(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	oldDate := time.Now().AddDate(0, 0, 1)
	book := &servicelib.Book{ID: bookID, DayPenalty: 10}
	nonReturnedBook := &servicelib.Book{ID: "654321", DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(-1 * time.Minute)}}

	customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: 20}
	libraryService := new(mocks.LibraryService)
	libraryService.On("GetBook", bookID).Return(book)
	libraryService.On("GetCustomer", customerID).Return(customer, nil)
	libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{nonReturnedBook}, nil)
	libraryService.On("CollectPayment", customerID, nonReturnedBook.DayPenalty).Return(nil)
	libraryService.On("SaveBook", nonReturnedBook).Return(nil)
	libraryService.On("SaveBook", book).Return(nil)

	err := LendBook(bookID, customerID, libraryService)
	assert.Nil(t, err)

	assert.NotEqual(t, oldDate, book.CurrentLend.LatestReturnDate)
	assert.True(t, oldDate.AddDate(0, 0, 6).Before(book.CurrentLend.LatestReturnDate))

	libraryService.AssertExpectations(t)
}

func TestCollectPaymentMultiple(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	testCases := []struct {
		age             int
		book1DayPrice   int
		book1Days       int
		book2DayPrice   int
		book2Days       int
		expectedPayment int
	}{
		{18, 10, 1, 10, 1, 20},
		{26, 10, 2, 10, 1, 30},
		{59, 5, 2, 5, 1, 15},
		{93, 1, 20, 3, 7, 41},
	}

	for _, tt := range testCases {
		book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().AddDate(0, 0, 1)}}
		nonReturnedBook1 := &servicelib.Book{ID: "654321", DayPenalty: tt.book1DayPrice, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(1*time.Hour).AddDate(0, 0, -tt.book1Days)}}
		nonReturnedBook2 := &servicelib.Book{ID: "765432", DayPenalty: tt.book2DayPrice, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(1*time.Hour).AddDate(0, 0, -tt.book2Days)}}

		libraryService := new(mocks.LibraryService)
		customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: tt.age}
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book, nonReturnedBook1, nonReturnedBook2}, nil)
		libraryService.On("CollectPayment", customerID, tt.expectedPayment).Return(nil)
		libraryService.On("SaveBook", nonReturnedBook1).Return(nil)
		libraryService.On("SaveBook", nonReturnedBook2).Return(nil)
		libraryService.On("SaveBook", book).Return(nil)

		err := LendBook(bookID, customerID, libraryService)
		assert.Nil(t, err)

		libraryService.AssertExpectations(t)
	}
}

func TestCollectPaymentMultipleYoungCustomer(t *testing.T) {
	bookID := "12345"
	customerID := 123456

	testCases := []struct {
		age             int
		book1DayPrice   int
		book1Days       int
		book2DayPrice   int
		book2Days       int
		expectedPayment int
	}{
		{13, 10, 1, 10, 1, 10},
		{16, 10, 2, 10, 1, 15},
		{17, 5, 2, 5, 1, 8},
		{15, 1, 20, 3, 7, 21},
	}

	for _, tt := range testCases {
		book := &servicelib.Book{ID: bookID, DayPenalty: 10, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().AddDate(0, 0, 1)}}
		nonReturnedBook1 := &servicelib.Book{ID: "654321", DayPenalty: tt.book1DayPrice, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(1*time.Hour).AddDate(0, 0, -tt.book1Days)}}
		nonReturnedBook2 := &servicelib.Book{ID: "765432", DayPenalty: tt.book2DayPrice, CurrentLend: &servicelib.Lend{CustomerID: customerID, LatestReturnDate: time.Now().Add(1*time.Hour).AddDate(0, 0, -tt.book2Days)}}

		libraryService := new(mocks.LibraryService)
		customer := &servicelib.Customer{ID: customerID, IsLocked: false, Age: tt.age}
		libraryService.On("GetBook", bookID).Return(book)
		libraryService.On("GetCustomer", customerID).Return(customer, nil)
		libraryService.On("GetLendsForCustomer", customerID).Return([]*servicelib.Book{book, nonReturnedBook1, nonReturnedBook2}, nil)
		libraryService.On("CollectPayment", customerID, tt.expectedPayment).Return(nil)
		libraryService.On("SaveBook", nonReturnedBook1).Return(nil)
		libraryService.On("SaveBook", nonReturnedBook2).Return(nil)
		libraryService.On("SaveBook", book).Return(nil)

		err := LendBook(bookID, customerID, libraryService)
		assert.Nil(t, err)

		libraryService.AssertExpectations(t)
	}
}
