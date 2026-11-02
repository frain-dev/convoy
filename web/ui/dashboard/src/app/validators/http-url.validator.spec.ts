import { FormControl, Validators } from '@angular/forms';
import { httpUrlValidator } from './http-url.validator';

describe('HTTP URL validation', () => {
	const endpoint = 'https://deploy-api.dev.example.com/deployment-service/v1/github/webhooks/deployment-protection';
	it('accepts endpoint, notification, private-host and URL-template shapes unchanged', () => {
		for (const url of [endpoint, 'http://localhost:8080/webhook', 'https://[::1]:8443/callback', 'https://example.com/orders/{reference}?key={key}', 'https://example.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?sig=redacted']) {
			expect(new FormControl(url, httpUrlValidator).valid).withContext(url).toBe(true);
		}
	});
	it('rejects pasted whitespace, quotes and long malformed URLs promptly', () => {
		const start = performance.now();
		for (const url of [endpoint + ' ', endpoint + '"', endpoint + '\n', 'https://' + 'a'.repeat(10000) + ' ', 'https://example.com:invalid/path', 'https:///path', 'ftp://example.com/path', 'javascript:alert(1)']) {
			expect(new FormControl(url, httpUrlValidator).invalid).withContext(url.slice(0, 100)).toBe(true);
		}
		expect(performance.now() - start).toBeLessThan(500);
	});
	it('keeps notification fields optional and endpoint URLs required', () => {
		expect(new FormControl('', httpUrlValidator).valid).toBe(true);
		expect(new FormControl('', [Validators.required, httpUrlValidator]).hasError('required')).toBe(true);
	});
});
