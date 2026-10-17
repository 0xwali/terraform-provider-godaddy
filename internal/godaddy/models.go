package godaddy

import "time"

type Domain struct {
	Domain              string     `json:"domain"`
	DomainID            int        `json:"domainId"`
	Status              string     `json:"status"`
	Expires             *time.Time `json:"expires,omitempty"`
	ExpirationProtected bool       `json:"expirationProtected"`
	HoldRegistrar       bool       `json:"holdRegistrar"`
	Locked              bool       `json:"locked"`
	Privacy             bool       `json:"privacy"`
	RenewAuto           bool       `json:"renewAuto"`
	RenewDeadline       *time.Time `json:"renewDeadline,omitempty"`
	Renewable           bool       `json:"renewable,omitempty"`
	TransferProtected   bool       `json:"transferProtected"`
	CreatedAt           time.Time  `json:"createdAt"`
	ModifiedAt          time.Time  `json:"modifiedAt,omitempty"`
	RegistrarCreatedAt  *time.Time `json:"registrarCreatedAt,omitempty"`
}

type DomainDetail struct {
	Domain
	ContactAdmin           DomainContact `json:"contactAdmin"`
	ContactBilling         DomainContact `json:"contactBilling"`
	ContactRegistrant      DomainContact `json:"contactRegistrant"`
	ContactTech            DomainContact `json:"contactTech"`
	Nameservers            []string      `json:"nameServers,omitempty"`
	DeletedAt              *time.Time    `json:"deletedAt,omitempty"`
	TransferAwayEligibleAt *time.Time    `json:"transferAwayEligibleAt,omitempty"`
	DNSSec                 *DNSSec       `json:"dnssec,omitempty"`
}

type DomainContact struct {
	NameFirst      string        `json:"nameFirst"`
	NameMiddle     string        `json:"nameMiddle,omitempty"`
	NameLast       string        `json:"nameLast"`
	Organization   string        `json:"organization,omitempty"`
	JobTitle       string        `json:"jobTitle,omitempty"`
	Email          string        `json:"email"`
	Phone          string        `json:"phone"`
	Fax            string        `json:"fax,omitempty"`
	AddressMailing DomainAddress `json:"addressMailing"`
}

type DomainAddress struct {
	Address1   string `json:"address1"`
	Address2   string `json:"address2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

type DNSSec struct {
	Enabled bool        `json:"enabled"`
	Keys    []DNSSecKey `json:"keys,omitempty"`
}

type DNSSecKey struct {
	Algorithm int    `json:"algorithm"`
	Flags     int    `json:"flags"`
	Protocol  int    `json:"protocol"`
	PublicKey string `json:"publicKey"`
}

type DNSRecord struct {
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	Data     string  `json:"data"`
	TTL      int     `json:"ttl"`
	Priority *int    `json:"priority,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Weight   *int    `json:"weight,omitempty"`
	Service  *string `json:"service,omitempty"`
	Protocol *string `json:"protocol,omitempty"`
}

type DomainAvailability struct {
	Available  bool   `json:"available"`
	Currency   string `json:"currency,omitempty"`
	Definitive bool   `json:"definitive"`
	Domain     string `json:"domain"`
	Period     int    `json:"period,omitempty"`
	Price      int    `json:"price,omitempty"`
}

type DomainPurchase struct {
	Consent           DomainConsent `json:"consent"`
	ContactAdmin      DomainContact `json:"contactAdmin"`
	ContactBilling    DomainContact `json:"contactBilling"`
	ContactRegistrant DomainContact `json:"contactRegistrant"`
	ContactTech       DomainContact `json:"contactTech"`
	Domain            string        `json:"domain"`
	NameServers       []string      `json:"nameServers,omitempty"`
	Period            int           `json:"period"`
	Privacy           bool          `json:"privacy"`
	RenewAuto         bool          `json:"renewAuto"`
}

type DomainConsent struct {
	AgreedAt      string   `json:"agreedAt"`
	AgreedBy      string   `json:"agreedBy"`
	AgreementKeys []string `json:"agreementKeys"`
}

type DomainUpdate struct {
	Locked       *bool    `json:"locked,omitempty"`
	NameServers  []string `json:"nameServers,omitempty"`
	RenewAuto    *bool    `json:"renewAuto,omitempty"`
	SubaccountId *string  `json:"subaccountId,omitempty"`
	ExposeWhois  *bool    `json:"exposeWhois,omitempty"`
}

type DNSRecordSet struct {
	Type    string      `json:"type"`
	Name    string      `json:"name"`
	Records []DNSRecord `json:"records"`
}
