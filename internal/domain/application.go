package domain

type ApplicationStatus string

const (
	ApplicationToReview  ApplicationStatus = "incelenecek"
	ApplicationSubmitted ApplicationStatus = "basvuruldu"
	ApplicationInterview ApplicationStatus = "sinav_mulakat"
	ApplicationPositive  ApplicationStatus = "olumlu"
	ApplicationNegative  ApplicationStatus = "olumsuz"
)
