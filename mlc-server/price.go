package main

func GetMLCDiscountPercent(sum CashCount) CashCount {
	if sum >= 65000 {
		return 25
	} else if sum >= 45000 {
		return 20
	} else if sum >= 35000 {
		return 15
	} else if sum >= 25000 {
		return 10
	}

	return 5
}
