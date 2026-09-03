package matching

import (
	"github.com/muzaffer/internship-tracker/internal/domain"
)

// NotificationOpportunity reports whether an opportunity type is a student
// opportunity that may reach the push-notification layer. Full-time and
// senior roles never qualify, regardless of score or source trust.
func NotificationOpportunity(kind domain.OpportunityType) bool {
	return notificationOpportunity(kind)
}
