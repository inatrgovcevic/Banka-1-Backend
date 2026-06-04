package dto_test

import (
	"encoding/json"
	"testing"

	"Banka1Back/credit-service-go/internal/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoanRequestDTO_UnmarshalJSON_IntRepaymentPeriod(t *testing.T) {
	t.Parallel()
	raw := `{"repaymentPeriod": 24}`
	var d dto.LoanRequestDTO
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	assert.Equal(t, 24, d.RepaymentPeriod)
}

func TestLoanRequestDTO_UnmarshalJSON_StringRepaymentPeriod(t *testing.T) {
	t.Parallel()
	raw := `{"repaymentPeriod": "36"}`
	var d dto.LoanRequestDTO
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	assert.Equal(t, 36, d.RepaymentPeriod)
}

func TestLoanRequestDTO_UnmarshalJSON_InvalidStringRepaymentPeriod(t *testing.T) {
	t.Parallel()
	raw := `{"repaymentPeriod": "notanumber"}`
	var d dto.LoanRequestDTO
	err := json.Unmarshal([]byte(raw), &d)
	require.Error(t, err)
}

func TestLoanRequestDTO_UnmarshalJSON_NullRepaymentPeriod(t *testing.T) {
	t.Parallel()
	raw := `{}`
	var d dto.LoanRequestDTO
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	assert.Equal(t, 0, d.RepaymentPeriod)
}
