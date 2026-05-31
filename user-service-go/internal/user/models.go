package user

import "time"

type ActorKind string

const (
	ActorEmployee ActorKind = "EMPLOYEE"
	ActorClient   ActorKind = "CLIENT"
)

type Employee struct {
	ID                  int64
	Ime                 string
	Prezime             string
	DatumRodjenja       time.Time
	Pol                 string
	Email               string
	BrojTelefona        *string
	Adresa              *string
	Username            string
	PasswordHash        *string
	Pozicija            string
	Departman           string
	Aktivan             bool
	Role                string
	FailedLoginAttempts int
	LockedUntil         *time.Time
	Permissions         []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Client struct {
	ID            int64
	Ime           string
	Prezime       string
	DatumRodjenja int64
	Pol           string
	Email         string
	BrojTelefona  *string
	Adresa        *string
	PasswordHash  *string
	JMBG          *string
	JMBGEncrypted *string
	Aktivan       bool
	Role          string
	Permissions   []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenResponse struct {
	JWT          string   `json:"jwt"`
	RefreshToken string   `json:"refreshToken,omitempty"`
	Role         string   `json:"role,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
}

type ClientLoginResponse struct {
	Token   string `json:"token"`
	ID      int64  `json:"id"`
	Ime     string `json:"ime"`
	Prezime string `json:"prezime"`
	Email   string `json:"email"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type ActivateRequest struct {
	ID                int64  `json:"id"`
	ConfirmationToken string `json:"confirmationToken"`
	Token             string `json:"token"`
	Password          string `json:"password"`
}

type EmailRequest struct {
	Email string `json:"email"`
}

type CheckActivateResponse struct {
	ID int64 `json:"id"`
}

type EmployeeCreateRequest struct {
	Ime           string `json:"ime"`
	Prezime       string `json:"prezime"`
	DatumRodjenja string `json:"datumRodjenja"`
	Pol           string `json:"pol"`
	Email         string `json:"email"`
	BrojTelefona  string `json:"brojTelefona"`
	Adresa        string `json:"adresa"`
	Username      string `json:"username"`
	Pozicija      string `json:"pozicija"`
	Departman     string `json:"departman"`
	Aktivan       *bool  `json:"aktivan"`
	Role          string `json:"role"`
}

type EmployeeUpdateRequest struct {
	Ime          *string `json:"ime"`
	Prezime      *string `json:"prezime"`
	Email        *string `json:"email"`
	BrojTelefona *string `json:"brojTelefona"`
	Adresa       *string `json:"adresa"`
	Pozicija     *string `json:"pozicija"`
	Departman    *string `json:"departman"`
	Aktivan      *bool   `json:"aktivan"`
	Role         *string `json:"role"`
	Margin       *bool   `json:"margin"`
}

type EmployeeResponse struct {
	ID            int64     `json:"id"`
	Ime           string    `json:"ime"`
	Prezime       string    `json:"prezime"`
	DatumRodjenja string    `json:"datumRodjenja,omitempty"`
	Pol           string    `json:"pol"`
	Email         string    `json:"email"`
	BrojTelefona  *string   `json:"brojTelefona,omitempty"`
	Adresa        *string   `json:"adresa,omitempty"`
	Username      string    `json:"username"`
	Pozicija      string    `json:"pozicija"`
	Departman     string    `json:"departman"`
	Aktivan       bool      `json:"aktivan"`
	Role          string    `json:"role"`
	Permissions   []string  `json:"permissions"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ClientCreateRequest struct {
	Ime           string `json:"ime"`
	Prezime       string `json:"prezime"`
	DatumRodjenja int64  `json:"datumRodjenja"`
	Pol           string `json:"pol"`
	Email         string `json:"email"`
	BrojTelefona  string `json:"brojTelefona"`
	Adresa        string `json:"adresa"`
	JMBG          string `json:"jmbg"`
	Role          string `json:"role"`
}

type ClientUpdateRequest struct {
	Ime          *string `json:"ime"`
	Prezime      *string `json:"prezime"`
	Email        *string `json:"email"`
	BrojTelefona *string `json:"brojTelefona"`
	Adresa       *string `json:"adresa"`
	Role         *string `json:"role"`
}

type ClientResponse struct {
	ID            int64     `json:"id"`
	Ime           string    `json:"ime"`
	Prezime       string    `json:"prezime"`
	DatumRodjenja int64     `json:"datumRodjenja"`
	Pol           string    `json:"pol"`
	Email         string    `json:"email"`
	BrojTelefona  *string   `json:"brojTelefona,omitempty"`
	Adresa        *string   `json:"adresa,omitempty"`
	Role          string    `json:"role"`
	Aktivan       bool      `json:"aktivan"`
	Permissions   []string  `json:"permissions"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ClientInfoResponse struct {
	ID            int64   `json:"id"`
	Ime           string  `json:"ime"`
	Prezime       string  `json:"prezime"`
	Name          string  `json:"name"`
	LastName      string  `json:"lastName"`
	Email         string  `json:"email,omitempty"`
	JMBG          *string `json:"jmbg,omitempty"`
	BrojTelefona  *string `json:"brojTelefona,omitempty"`
	PhoneNumber   *string `json:"phoneNumber,omitempty"`
	Adresa        *string `json:"adresa,omitempty"`
	Address       *string `json:"address,omitempty"`
	Pol           string  `json:"pol,omitempty"`
	Gender        string  `json:"gender,omitempty"`
	DatumRodjenja int64   `json:"datumRodjenja,omitempty"`
	DateOfBirth   int64   `json:"dateOfBirth,omitempty"`
	Role          string  `json:"role,omitempty"`
	Aktivan       bool    `json:"aktivan"`
	Active        bool    `json:"active"`
}

type InterbankUserDisplayResponse struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
}

type PageResponse[T any] struct {
	Content          []T  `json:"content"`
	TotalElements    int  `json:"totalElements"`
	TotalPages       int  `json:"totalPages"`
	CurrentPage      int  `json:"currentPage"`
	Number           int  `json:"number"`
	Size             int  `json:"size"`
	NumberOfElements int  `json:"numberOfElements"`
	First            bool `json:"first"`
	Last             bool `json:"last"`
	Empty            bool `json:"empty"`
}

func employeeDTO(employee Employee) EmployeeResponse {
	return EmployeeResponse{
		ID:            employee.ID,
		Ime:           employee.Ime,
		Prezime:       employee.Prezime,
		DatumRodjenja: employee.DatumRodjenja.Format("2006-01-02"),
		Pol:           employee.Pol,
		Email:         employee.Email,
		BrojTelefona:  employee.BrojTelefona,
		Adresa:        employee.Adresa,
		Username:      employee.Username,
		Pozicija:      employee.Pozicija,
		Departman:     employee.Departman,
		Aktivan:       employee.Aktivan,
		Role:          employee.Role,
		Permissions:   employee.Permissions,
		CreatedAt:     employee.CreatedAt,
		UpdatedAt:     employee.UpdatedAt,
	}
}

func clientDTO(client Client) ClientResponse {
	return ClientResponse{
		ID:            client.ID,
		Ime:           client.Ime,
		Prezime:       client.Prezime,
		DatumRodjenja: client.DatumRodjenja,
		Pol:           client.Pol,
		Email:         client.Email,
		BrojTelefona:  client.BrojTelefona,
		Adresa:        client.Adresa,
		Role:          client.Role,
		Aktivan:       client.Aktivan,
		Permissions:   client.Permissions,
		CreatedAt:     client.CreatedAt,
		UpdatedAt:     client.UpdatedAt,
	}
}
