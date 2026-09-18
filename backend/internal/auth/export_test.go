package auth

// Test seams: let the external auth_test package drive the real device-code
// handler against a mock GitHub server and pre-register a device code.

func RememberDeviceCodeForTest(code string) { rememberDeviceCode(code) }

func SetGitHubURLsForTest(token, user, emails string) {
	githubTokenURL, githubUserURL, githubUserEmailsURL = token, user, emails
}
