package template

import (
	"fmt"

	"Banka1Back/notification-service-go/internal/model"
)

// EmailTemplate holds the raw subject and body template for one notification type.
// Both fields may contain {{key}} placeholders.
type EmailTemplate struct {
	Subject      string
	BodyTemplate string
}

// TemplateRegistry maps a NotificationType to its EmailTemplate.
// Constructed once at startup and treated as read-only thereafter.
type TemplateRegistry struct {
	templates map[string]EmailTemplate
}

// NewDefaultTemplateRegistry returns a TemplateRegistry pre-loaded with all templates.
// Individual entries can be overridden by passing override entries (used for testing).
func NewDefaultTemplateRegistry(overrides map[string]EmailTemplate) *TemplateRegistry {
	merged := make(map[string]EmailTemplate, len(defaultTemplates)+len(overrides))
	for k, v := range defaultTemplates {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return &TemplateRegistry{templates: merged}
}

// Resolve returns the EmailTemplate for the given notification type.
func (r *TemplateRegistry) Resolve(notificationType model.NotificationType) (EmailTemplate, error) {
	tmpl, ok := r.templates[string(notificationType)]
	if !ok {
		return EmailTemplate{}, fmt.Errorf(
			"ERR_NOTIFICATION_004: no template defined for notification type %q", notificationType,
		)
	}
	return tmpl, nil
}

var defaultTemplates = map[string]EmailTemplate{
	"EMPLOYEE_CREATED": {
		Subject:      "Activation Email",
		BodyTemplate: "Zdravo {{name}}, vas nalog je kreiran. Aktivirajte nalog klikom na link:\n{{activationLink}}",
	},
	"EMPLOYEE_PASSWORD_RESET": {
		Subject:      "Password Reset Email",
		BodyTemplate: "Zdravo {{name}}, resetujte lozinku klikom na link:\n{{resetLink}}",
	},
	"EMPLOYEE_ACCOUNT_DEACTIVATED": {
		Subject:      "Account Deactivation Email",
		BodyTemplate: "Zdravo {{name}}, vas nalog je deaktiviran.",
	},
	"CLIENT_CREATED": {
		Subject:      "Client Account Activation Email",
		BodyTemplate: "Zdravo {{name}}, vas klijentski nalog je kreiran. Aktivirajte nalog klikom na link:\n{{activationLink}}",
	},
	"CLIENT_PASSWORD_RESET": {
		Subject:      "Client Password Reset Email",
		BodyTemplate: "Zdravo {{name}}, resetujte lozinku klikom na link:\n{{resetLink}}",
	},
	"CLIENT_ACCOUNT_DEACTIVATED": {
		Subject:      "Client Account Deactivation Email",
		BodyTemplate: "Zdravo {{name}}, vas klijentski nalog je deaktiviran.",
	},
	"VERIFICATION_OTP": {
		Subject:      "Verifikacioni kod",
		BodyTemplate: "Vas verifikacioni kod je: {{code}}",
	},
	"CARD_REQUEST_VERIFICATION": {
		Subject:      "Card Verification Code",
		BodyTemplate: "Zdravo {{name}}, kod za verifikaciju zahteva za karticu {{cardName}} na racunu {{accountNumber}} je: {{verificationCode}}",
	},
	"CARD_REQUEST_SUCCESS": {
		Subject:      "Card Request Completed",
		BodyTemplate: "Zdravo {{name}}, uspesno je kreirana vasa kartica {{cardName}} za racun {{accountNumber}} sa brojem {{cardNumber}}.",
	},
	"CARD_REQUEST_FAILURE": {
		Subject:      "Card Request Failed",
		BodyTemplate: "Zdravo {{name}}, zahtev za kreiranje kartice {{cardName}} za racun {{accountNumber}} nije uspesno zavrsen. Razlog: {{reason}}",
	},
	"CARD_BLOCKED": {
		Subject:      "Card Blocked",
		BodyTemplate: "Zdravo {{name}}, vasa kartica {{cardName}} za racun {{accountNumber}} sa brojem {{cardNumber}} je blokirana.",
	},
	"CARD_UNBLOCKED": {
		Subject:      "Card Unblocked",
		BodyTemplate: "Zdravo {{name}}, vasa kartica {{cardName}} za racun {{accountNumber}} sa brojem {{cardNumber}} je ponovo aktivna.",
	},
	"CARD_DEACTIVATED": {
		Subject:      "Card Deactivated",
		BodyTemplate: "Zdravo {{name}}, vasa kartica {{cardName}} za racun {{accountNumber}} sa brojem {{cardNumber}} je deaktivirana.",
	},
	"CREDIT_REQUESTED": {
		Subject:      "Zahtev za kredit primljen",
		BodyTemplate: "Zdravo {{name}}, vas zahtev za kredit je uspesno podnet i ceka na obradu.",
	},
	"CREDIT_APPROVED": {
		Subject:      "Credit Request Approved",
		BodyTemplate: "Zdravo {{name}}, vas kreditni zahtev sa identifikatorom {{creditId}} je odobren. Odobren iznos: {{approvedAmount}}.",
	},
	"CREDIT_DECLINED": {
		Subject:      "Credit Request Declined",
		BodyTemplate: "Zdravo {{name}}, vas kreditni zahtev sa identifikatorom {{creditId}} je odbijen.",
	},
	"CREDIT_INSTALLMENT_FAILED": {
		Subject:      "Installment Charge Failed",
		BodyTemplate: "Zdravo {{name}}, naplata rate za kredit {{creditId}} nije uspela. Dospeli iznos: {{installmentAmount}}. Pokusacemo ponovo za {{hours}}h.",
	},
	"ORDER_APPROVED": {
		Subject:      "Order Approved",
		BodyTemplate: "Zdravo {{name}}, order {{orderId}} za listing {{listingId}} je odobren. Tip: {{orderType}}, smer: {{direction}}, supervisor: {{supervisorId}}.",
	},
	"ORDER_DECLINED": {
		Subject:      "Order Declined",
		BodyTemplate: "Zdravo {{name}}, order {{orderId}} za listing {{listingId}} je odbijen. Tip: {{orderType}}, smer: {{direction}}, supervisor: {{supervisorId}}.",
	},
	"TAX_COLLECTED": {
		Subject:      "Tax Collected",
		BodyTemplate: "Zdravo {{name}}, naplacen je porez za listing {{listingId}}. Transakcija: {{transactionId}}, porez: {{tax}}, porez u RSD: {{taxRsd}}.",
	},
	"OTC_COUNTER_OFFERED": {
		Subject:      "OTC Counter Offer",
		BodyTemplate: "Zdravo {{name}}, druga strana je poslala kontraponudu za OTC pregovor {{offerId}}. Ticker: {{stockTicker}}, kolicina: {{amount}}, cena: {{pricePerStock}}, premija: {{premium}}, status: {{status}}, rok: {{expiryDate}}, vreme: {{timestamp}}.",
	},
	"OTC_ACCEPTED": {
		Subject:      "OTC Offer Accepted",
		BodyTemplate: "Zdravo {{name}}, OTC pregovor {{offerId}} je prihvacen. Ticker: {{stockTicker}}, kolicina: {{amount}}, cena: {{pricePerStock}}, status: {{status}}, rok: {{expiryDate}}, vreme: {{timestamp}}.",
	},
	"OTC_CANCELED": {
		Subject:      "OTC Offer Closed",
		BodyTemplate: "Zdravo {{name}}, OTC pregovor {{offerId}} je zatvoren. Dogadjaj: {{eventType}}, ticker: {{stockTicker}}, status: {{status}}, rok: {{expiryDate}}, vreme: {{timestamp}}.",
	},
	"OTC_EXPIRY_REMINDER": {
		Subject:      "OTC Contract Expiry Reminder",
		BodyTemplate: "Zdravo {{name}}, OTC ugovor {{contractId}} za ticker {{stockTicker}} istice za {{reminderDays}} dana, na datum {{expiryDate}}. Status: {{status}}, vreme obavestenja: {{timestamp}}.",
	},
	"ACCOUNT_CREATED": {
		Subject:      "Otvaranje računa",
		BodyTemplate: "Zdravo {{name}}, vas bancarski racun je uspesno otvoren.",
	},
	"ACCOUNT_DEACTIVATED": {
		Subject:      "Deaktivacija računa",
		BodyTemplate: "Zdravo {{name}}, vas bancarski racun je deaktiviran.",
	},
	"TRANSACTION_COMPLETED": {
		Subject:      "Potvrda plaćanja",
		BodyTemplate: "Zdravo {{name}}, vaseplacanje je uspesno izvrseno.",
	},
	"TRANSACTION_DENIED": {
		Subject:      "Plaćanje odbijeno",
		BodyTemplate: "Zdravo {{name}}, vase placanje nije moglo biti izvrseno.",
	},
	"PRICE_ALERT_TRIGGERED": {
		Subject:      "Cenovni alarm aktiviran",
		BodyTemplate: "Zdravo {{name}}, cena za {{ticker}} je dostigla {{price}}.",
	},
	"ORDER_RECURRING_SKIPPED": {
		Subject:      "Periodicni nalog preskocen",
		BodyTemplate: "Zdravo {{name}}, vas periodicni nalog {{orderId}} nije izvrsen. Razlog: {{reason}}.",
	},
	"ORDER_CREATED": {
		Subject:      "Nalog kreiran",
		BodyTemplate: "Zdravo {{name}}, vas nalog {{orderId}} ({{ticker}}) je kreiran. Smer: {{direction}}, kolicina: {{quantity}}, cena: {{price}}. Status: {{status}}.",
	},
	"ORDER_DONE": {
		Subject:      "Nalog izvrsen",
		BodyTemplate: "Zdravo {{name}}, vas nalog {{orderId}} ({{ticker}}) je u potpunosti izvrsen. Kolicina: {{quantity}}, cena izvrsenja: {{price}}.",
	},
	"ORDER_PARTIAL_FILL": {
		Subject:      "Delimicno izvrsenje naloga",
		BodyTemplate: "Zdravo {{name}}, nalog {{orderId}} ({{ticker}}) je delimicno izvrsen. Preostala kolicina: {{remainingPortions}}, cena: {{price}}.",
	},
	"ORDER_AUTO_CANCELLED": {
		Subject:      "Nalog otkazan",
		BodyTemplate: "Zdravo {{name}}, vas nalog {{orderId}} ({{ticker}}) je automatski otkazan jer je istekao rok poravnanja.",
	},
}
