package main

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/olekukonko/errors"
)

type UserForm struct {
	Name     string
	Email    string
	Password string
	Birthday string
}

func validateUser(form UserForm) *errors.MultiError {
	multi := errors.NewMultiError(
		errors.WithLimit(10),
		errors.WithFormatter(customFormat),
	)

	// Name validation
	if form.Name == "" {
		multi.Add(errors.New("name is required"))
	} else if len(form.Name) > 50 {
		multi.Add(errors.New("name cannot exceed 50 characters"))
	}

	// Email validation
	if form.Email == "" {
		multi.Add(errors.New("email is required"))
	} else {
		if _, err := mail.ParseAddress(form.Email); err != nil {
			multi.Add(errors.New("invalid email format"))
		}
		if !strings.Contains(form.Email, "@") {
			multi.Add(errors.New("email must contain @ symbol"))
		}
	}

	// Password validation
	if len(form.Password) < 8 {
		multi.Add(errors.New("password must be at least 8 characters"))
	}
	if !strings.ContainsAny(form.Password, "0123456789") {
		multi.Add(errors.New("password must contain at least one number"))
	}
	if !strings.ContainsAny(form.Password, "!@#$%^&*") {
		multi.Add(errors.New("password must contain at least one special character"))
	}

	// Birthday validation
	if form.Birthday != "" {
		if _, err := time.Parse("2006-01-02", form.Birthday); err != nil {
			multi.Add(errors.New("birthday must be in YYYY-MM-DD format"))
		} else if bday, _ := time.Parse("2006-01-02", form.Birthday); time.Since(bday).Hours()/24/365 < 13 {
			multi.Add(errors.New("must be at least 13 years old"))
		}
	}

	return multi
}

func customFormat(errs []error) string {
	var sb strings.Builder
	sb.WriteString("🚨 Validation Errors:\n")
	for i, err := range errs {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err))
	}
	sb.WriteString(fmt.Sprintf("\nTotal issues found: %d\n", len(errs)))
	return sb.String()
}

func main() {
	fmt.Println("=== User Registration Validation ===")

	user := UserForm{
		Name:     "", // Empty name
		Email:    "invalid-email",
		Password: "weak",
		Birthday: "2015-01-01", // Under 13
	}

	// Generate multiple validation errors
	validationErrors := validateUser(user)

	if validationErrors.Has() {
		fmt.Println(validationErrors)

		// Detailed error analysis
		fmt.Println("\n🔍 Error Analysis:")
		fmt.Printf("Total errors: %d\n", validationErrors.Count())
		fmt.Printf("First error: %v\n", validationErrors.First())
		fmt.Printf("Last error: %v\n", validationErrors.Last())

		// Categorized errors with consistent formatting
		fmt.Println("\n📋 Error Categories:")
		if emailErrors := validationErrors.Filter(contains("email")); emailErrors.Has() {
			fmt.Println("Email Issues:")
			if emailErrors.Count() == 1 {
				fmt.Println(customFormat([]error{emailErrors.First()}))
			} else {
				fmt.Println(emailErrors)
			}
		}
		if pwErrors := validationErrors.Filter(contains("password")); pwErrors.Has() {
			fmt.Println("Password Issues:")
			if pwErrors.Count() == 1 {
				fmt.Println(customFormat([]error{pwErrors.First()}))
			} else {
				fmt.Println(pwErrors)
			}
		}
		if ageErrors := validationErrors.Filter(contains("13 years")); ageErrors.Has() {
			fmt.Println("Age Restriction:")
			if ageErrors.Count() == 1 {
				fmt.Println(customFormat([]error{ageErrors.First()}))
			} else {
				fmt.Println(ageErrors)
			}
		}
	}

	// System Error Aggregation Example
	fmt.Println("\n=== System Error Aggregation ===")
	systemErrors := errors.NewMultiError(
		errors.WithLimit(5),
		errors.WithFormatter(systemErrorFormat),
	)

	// Simulate system errors
	systemErrors.Add(errors.New("database connection timeout").WithRetryable())
	systemErrors.Add(errors.New("API rate limit exceeded").WithRetryable())
	systemErrors.Add(errors.New("disk space low"))
	systemErrors.Add(errors.New("database connection timeout").WithRetryable()) // Duplicate
	systemErrors.Add(errors.New("cache miss"))
	systemErrors.Add(errors.New("database connection timeout").WithRetryable()) // Over limit

	fmt.Println(systemErrors)
	fmt.Printf("\nSystem Status: %d active issues\n", systemErrors.Count())

	// Filter retryable errors
	if retryable := systemErrors.Filter(errors.IsRetryable); retryable.Has() {
		fmt.Println("\n🔄 Retryable Errors:")
		fmt.Println(retryable)
	}
}

func systemErrorFormat(errs []error) string {
	var sb strings.Builder
	sb.WriteString("⚠️ System Alerts:\n")
	for i, err := range errs {
		sb.WriteString(fmt.Sprintf("  %d. %s", i+1, err))
		if errors.IsRetryable(err) {
			sb.WriteString(" (retryable)")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func contains(substr string) func(error) bool {
	return func(err error) bool {
		return strings.Contains(err.Error(), substr)
	}
}
