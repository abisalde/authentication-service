package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database"
	"github.com/abisalde/authentication-service/internal/database/ent/user"
	"github.com/abisalde/authentication-service/pkg/password"
)

type MockUser struct {
	Email         string
	Username      string
	FirstName     string
	LastName      string
	PhoneNumber   string
	Provider      string
	Role          string
	StreetName    string
	City          string
	State         string
	ZipCode       string
	Country       string
	MarketingOpt  bool
	EmailVerified bool
}

var mockUsers = []MockUser{
	{
		Email: "john.doe@example.com", Username: "johndoe", FirstName: "John", LastName: "Doe",
		PhoneNumber: "+1234567890", Provider: "EMAIL", Role: "USER",
		StreetName: "123 Main St", City: "New York", State: "NY", ZipCode: "10001", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "jane.smith@example.com", Username: "janesmith", FirstName: "Jane", LastName: "Smith",
		PhoneNumber: "+1234567891", Provider: "EMAIL", Role: "USER",
		StreetName: "456 Oak Ave", City: "Los Angeles", State: "CA", ZipCode: "90001", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "admin.user@example.com", Username: "adminuser", FirstName: "Admin", LastName: "User",
		PhoneNumber: "+1234567892", Provider: "EMAIL", Role: "ADMIN",
		StreetName: "789 Admin Blvd", City: "Washington", State: "DC", ZipCode: "20001", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "bob.johnson@example.com", Username: "bobjohnson", FirstName: "Bob", LastName: "Johnson",
		PhoneNumber: "+1234567893", Provider: "EMAIL", Role: "USER",
		StreetName: "321 Elm St", City: "Chicago", State: "IL", ZipCode: "60601", Country: "USA",
		MarketingOpt: true, EmailVerified: false,
	},
	{
		Email: "alice.williams@example.com", Username: "alicewilliams", FirstName: "Alice", LastName: "Williams",
		PhoneNumber: "+1234567894", Provider: "GOOGLE", Role: "USER",
		StreetName: "654 Maple Dr", City: "Houston", State: "TX", ZipCode: "77001", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "charlie.brown@example.com", Username: "charliebrown", FirstName: "Charlie", LastName: "Brown",
		PhoneNumber: "+1234567895", Provider: "EMAIL", Role: "USER",
		StreetName: "987 Pine Rd", City: "Phoenix", State: "AZ", ZipCode: "85001", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "diana.jones@example.com", Username: "dianajones", FirstName: "Diana", LastName: "Jones",
		PhoneNumber: "+1234567896", Provider: "FACEBOOK", Role: "USER",
		StreetName: "147 Cedar Ln", City: "Philadelphia", State: "PA", ZipCode: "19101", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "edward.davis@example.com", Username: "edwarddavis", FirstName: "Edward", LastName: "Davis",
		PhoneNumber: "+1234567897", Provider: "EMAIL", Role: "USER",
		StreetName: "258 Birch Ave", City: "San Antonio", State: "TX", ZipCode: "78201", Country: "USA",
		MarketingOpt: true, EmailVerified: false,
	},
	{
		Email: "fiona.miller@example.com", Username: "fionamiller", FirstName: "Fiona", LastName: "Miller",
		PhoneNumber: "+1234567898", Provider: "EMAIL", Role: "USER",
		StreetName: "369 Spruce St", City: "San Diego", State: "CA", ZipCode: "92101", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "george.wilson@example.com", Username: "georgewilson", FirstName: "George", LastName: "Wilson",
		PhoneNumber: "+1234567899", Provider: "GOOGLE", Role: "USER",
		StreetName: "741 Walnut Blvd", City: "Dallas", State: "TX", ZipCode: "75201", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "helen.moore@example.com", Username: "helenmoore", FirstName: "Helen", LastName: "Moore",
		PhoneNumber: "+1234567800", Provider: "EMAIL", Role: "USER",
		StreetName: "852 Poplar Dr", City: "San Jose", State: "CA", ZipCode: "95101", Country: "USA",
		MarketingOpt: false, EmailVerified: false,
	},
	{
		Email: "isaac.taylor@example.com", Username: "isaactaylor", FirstName: "Isaac", LastName: "Taylor",
		PhoneNumber: "+1234567801", Provider: "EMAIL", Role: "USER",
		StreetName: "963 Ash Rd", City: "Austin", State: "TX", ZipCode: "73301", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "julia.anderson@example.com", Username: "juliaanderson", FirstName: "Julia", LastName: "Anderson",
		PhoneNumber: "+1234567802", Provider: "FACEBOOK", Role: "USER",
		StreetName: "159 Cypress Ln", City: "Jacksonville", State: "FL", ZipCode: "32099", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "kevin.thomas@example.com", Username: "kevinthomas", FirstName: "Kevin", LastName: "Thomas",
		PhoneNumber: "+1234567803", Provider: "EMAIL", Role: "ADMIN",
		StreetName: "357 Willow Ave", City: "San Francisco", State: "CA", ZipCode: "94101", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "laura.jackson@example.com", Username: "laurajackson", FirstName: "Laura", LastName: "Jackson",
		PhoneNumber: "+1234567804", Provider: "EMAIL", Role: "USER",
		StreetName: "486 Redwood St", City: "Columbus", State: "OH", ZipCode: "43004", Country: "USA",
		MarketingOpt: false, EmailVerified: false,
	},
	{
		Email: "michael.white@example.com", Username: "michaelwhite", FirstName: "Michael", LastName: "White",
		PhoneNumber: "+1234567805", Provider: "GOOGLE", Role: "USER",
		StreetName: "792 Magnolia Blvd", City: "Fort Worth", State: "TX", ZipCode: "76101", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
	{
		Email: "nancy.harris@example.com", Username: "nancyharris", FirstName: "Nancy", LastName: "Harris",
		PhoneNumber: "+1234567806", Provider: "EMAIL", Role: "USER",
		StreetName: "135 Dogwood Dr", City: "Charlotte", State: "NC", ZipCode: "28201", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "oliver.martin@example.com", FirstName: "Oliver", LastName: "Martin",
		PhoneNumber: "+1234567807", Provider: "EMAIL", Role: "USER",
		StreetName: "246 Hickory Rd", City: "Detroit", State: "MI", ZipCode: "48201", Country: "USA",
		MarketingOpt: true, EmailVerified: false,
	},
	{
		Email: "patricia.garcia@example.com", Username: "patriciagarcia", FirstName: "Patricia", LastName: "Garcia",
		PhoneNumber: "+1234567808", Provider: "FACEBOOK", Role: "USER",
		StreetName: "579 Beech Ln", City: "El Paso", State: "TX", ZipCode: "79901", Country: "USA",
		MarketingOpt: false, EmailVerified: true,
	},
	{
		Email: "quincy.martinez@example.com", Username: "quincymartinez", FirstName: "Quincy", LastName: "Martinez",
		PhoneNumber: "+1234567809", Provider: "EMAIL", Role: "USER",
		StreetName: "680 Fir Ave", City: "Seattle", State: "WA", ZipCode: "98101", Country: "USA",
		MarketingOpt: true, EmailVerified: true,
	},
}

func main() {
	ctx := context.Background()

	cfg, err := configs.Load("development")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Client.Close()

	count, err := db.Client.User.Query().Count(ctx)
	if err != nil {
		log.Fatalf("Failed to count users: %v", err)
	}

	if count > 0 {
		log.Printf("⚠️  Database already contains %d users. Skipping seed...", count)
		log.Println("To re-seed, please clear the users table first.")
		return
	}

	defaultPassword := "Password123!"
	hashedPassword, err := password.HashPassword(defaultPassword)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	log.Println("🌱 Starting to seed users...")

	successCount := 0
	for i, mockUser := range mockUsers {
		userID := int64(1000 + i)

		userCreate := db.Client.User.Create().
			SetID(userID).
			SetEmail(mockUser.Email).
			SetUsername(mockUser.Username).
			SetFirstName(mockUser.FirstName).
			SetLastName(mockUser.LastName).
			SetPhoneNumber(mockUser.PhoneNumber).
			SetStreetName(mockUser.StreetName).
			SetCity(mockUser.City).
			SetState(mockUser.State).
			SetZipCode(mockUser.ZipCode).
			SetCountry(mockUser.Country).
			SetMarketingOptIn(mockUser.MarketingOpt).
			SetIsEmailVerified(mockUser.EmailVerified).
			SetTermsAcceptedAt(time.Now().Add(-time.Duration(i) * 24 * time.Hour))

		switch mockUser.Provider {
		case "EMAIL":
			userCreate.SetPasswordHash(hashedPassword).SetProvider(user.ProviderEMAIL)
		case "GOOGLE":
			userCreate.SetOauthID(fmt.Sprintf("google_%s", mockUser.Username)).SetProvider(user.ProviderGOOGLE)
		case "FACEBOOK":
			userCreate.SetOauthID(fmt.Sprintf("facebook_%s", mockUser.Username)).SetProvider(user.ProviderFACEBOOK)
		}

		if mockUser.Role == "ADMIN" {
			userCreate.SetRole(user.RoleADMIN)
		} else {
			userCreate.SetRole(user.RoleUSER)
		}

		if mockUser.EmailVerified {
			userCreate.SetLastLoginAt(time.Now().Add(-time.Duration(i) * time.Hour))
		}

		createdUser, err := userCreate.Save(ctx)
		if err != nil {
			log.Printf("❌ Failed to create user %s: %v", mockUser.Email, err)
			continue
		}

		successCount++
		log.Printf("✅ Created user %d/%d: %s (%s)", successCount, len(mockUsers), createdUser.Email, createdUser.Username)
	}

	log.Printf("\n🎉 Seed completed! Successfully created %d/%d users", successCount, len(mockUsers))
	log.Printf("📝 Default password for EMAIL users: %s\n", defaultPassword)
}
