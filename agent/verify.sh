#!/bin/bash
# Verification script for the agent package

set -e

echo "=== Verifying Aixgo Agent Package ==="
echo ""

echo "1. Building package..."
go build ./agent
echo "✓ Package builds successfully"
echo ""

echo "2. Running tests..."
go test -v ./agent > /tmp/agent_test.log 2>&1
if [ $? -eq 0 ]; then
    echo "✓ All tests pass"
    echo "   $(grep -c 'PASS:' /tmp/agent_test.log) test cases passed"
else
    echo "✗ Tests failed"
    cat /tmp/agent_test.log
    exit 1
fi
echo ""

echo "3. Checking test coverage..."
COVERAGE=$(go test -cover ./agent 2>&1 | grep coverage | awk '{print $5}')
echo "✓ Test coverage: $COVERAGE"
echo ""

echo "4. Running go vet..."
go vet ./agent
echo "✓ No vet issues"
echo ""

echo "5. Running benchmarks..."
go test -bench=. -benchmem ./agent > /tmp/agent_bench.log 2>&1
echo "✓ Benchmarks complete:"
grep "^Benchmark" /tmp/agent_bench.log | head -4
echo ""

echo "6. Verifying documentation..."
FILES=("README.md" "EXPORTS.md" "INTEGRATION.md" "PACKAGE_SUMMARY.md")
for file in "${FILES[@]}"; do
    if [ -f "agent/$file" ]; then
        echo "✓ $file exists ($(wc -l < agent/$file) lines)"
    else
        echo "✗ $file missing"
        exit 1
    fi
done
echo ""

echo "7. Checking Go documentation..."
go doc agent > /dev/null 2>&1
echo "✓ Package documentation generated"
echo ""

echo "=== All Verifications Passed ==="
echo ""
echo "Package: github.com/aixgo-dev/aixgo/agent"
echo "Status: Ready for use"
echo ""
echo "Next steps:"
echo "  - Import in external projects: go get github.com/aixgo-dev/aixgo/agent"
echo "  - Read documentation: cat agent/README.md"
echo "  - View examples: go test -v -run Example ./agent"
