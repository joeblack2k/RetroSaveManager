.PHONY: test-contract test-golden verify-contract verify-contract-drift

test-contract:
	go test ./cmd/server -run '^TestContract' -count=1

test-golden:
	go test ./cmd/server -run '^TestGolden' -count=1

verify-contract:
	./scripts/verify-contract.sh

verify-contract-drift:
	./scripts/verify-contract-drift.sh
