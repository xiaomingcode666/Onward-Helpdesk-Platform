package request

// DemoRequestRequest is a public marketing lead submission. Website is a
// honeypot field and should be left empty by a human visitor.
type DemoRequestRequest struct {
	Company       string `json:"company" binding:"required,max=160"`
	ContactName   string `json:"contactName" binding:"required,max=100"`
	Email         string `json:"email" binding:"required,max=254"`
	Mobile        string `json:"mobile" binding:"max=50"`
	CountryRegion string `json:"countryRegion" binding:"max=100"`
	Requirements  string `json:"requirements" binding:"max=2000"`
	Website       string `json:"website" binding:"max=200"`
}
