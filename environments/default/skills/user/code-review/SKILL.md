---
name: code-review
description: Systematic code review process with focus on quality, security, and best practices
allowed-tools:
  - read_file
  - grep
  - glob
  - bash
triggers:
  - review
  - code quality
  - pull request
---

# Code Review Skill

## When to Use
- User asks for code review or feedback
- Reviewing a pull request or diff
- Analyzing code quality or security

## Review Checklist

### 1. Functionality
- [ ] Does the code do what it's supposed to?
- [ ] Are edge cases handled?
- [ ] Is error handling appropriate?

### 2. Code Quality
- [ ] Is the code readable and maintainable?
- [ ] Are functions and variables named clearly?
- [ ] Is there unnecessary duplication?
- [ ] Are comments helpful and accurate?

### 3. Security
- [ ] Input validation present?
- [ ] No hardcoded secrets?
- [ ] SQL injection prevention?
- [ ] XSS prevention?

### 4. Performance
- [ ] Efficient algorithms used?
- [ ] No N+1 query problems?
- [ ] Resources properly managed?

### 5. Testing
- [ ] Tests cover main functionality?
- [ ] Edge cases tested?
- [ ] Tests are readable?

## Output Format

For each finding, provide:
1. **Location**: File and line number
2. **Severity**: Critical / High / Medium / Low
3. **Issue**: Clear description of the problem
4. **Suggestion**: How to fix it
5. **Example**: Code snippet showing the fix (if applicable)

## Example Review Comment

**Location**: `src/api/users.go:42`
**Severity**: High
**Issue**: SQL query built using string concatenation
**Suggestion**: Use parameterized queries to prevent SQL injection
**Example**:
```go
// Before (vulnerable)
query := "SELECT * FROM users WHERE id = " + userID

// After (safe)
query := "SELECT * FROM users WHERE id = ?"
db.Query(query, userID)
```
