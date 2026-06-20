import {AbstractControl, ValidationErrors, ValidatorFn} from '@angular/forms';

// Basic zip code validation patterns for common countries.
const ZIP_PATTERNS: { [key: string]: RegExp } = {
	'US': /^\d{5}(-\d{4})?$/, // 12345 or 12345-6789
	'CA': /^[A-Za-z]\d[A-Za-z]\s?\d[A-Za-z]\d$/, // A1A 1A1
	'GB': /^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$/, // A1 1AA or AA1A 1AA
	'DE': /^\d{5}$/, // 12345
	'FR': /^\d{5}$/, // 12345
	'IT': /^\d{5}$/, // 12345
	'ES': /^\d{5}$/, // 12345
	'NL': /^\d{4}\s?[A-Z]{2}$/, // 1234 AB
	'BE': /^\d{4}$/, // 1234
	'AT': /^\d{4}$/, // 1234
	'DK': /^\d{4}$/, // 1234
	'SE': /^\d{3}\s?\d{2}$/, // 123 45
	'NO': /^\d{4}$/, // 1234
	'AU': /^\d{4}$/, // 1234
	'DEFAULT': /^[A-Z0-9\s\-]{3,10}$/ // Generic pattern for other countries
};

export function zipCodeValidator(): ValidatorFn {
	return (control: AbstractControl): ValidationErrors | null => {
		if (!control.value) {
			return null;
		}

		const zipCode = control.value.trim();

		for (const [, pattern] of Object.entries(ZIP_PATTERNS)) {
			if (pattern.test(zipCode)) {
				return null; // Valid zip code
			}
		}

		// If no specific pattern matches, use generic validation
		if (ZIP_PATTERNS.DEFAULT.test(zipCode)) {
			return null; // Acceptable format
		}

		return { invalidZipCode: true };
	};
}
