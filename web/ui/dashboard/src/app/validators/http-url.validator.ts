import { ValidatorFn } from '@angular/forms';

// URL parsing avoids nested hostname/path regexes that can block the UI while
// validating pasted, malformed URLs. Required fields use Validators.required.
export const httpUrlValidator: ValidatorFn = control => {
	const value = control.value;
	if (value === null || value === undefined || value === '') return null;
	const invalid = { pattern: { actualValue: value } };
	if (typeof value !== 'string' || /[\s\\<>"`]/.test(value)) return invalid;
	try {
		const url = new URL(value);
		return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname && /^https?:\/\/[^/]/i.test(value) ? null : invalid;
	} catch {
		return invalid;
	}
};
