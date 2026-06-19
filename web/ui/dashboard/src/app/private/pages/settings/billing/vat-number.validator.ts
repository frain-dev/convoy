import {AbstractControl, ValidationErrors, ValidatorFn} from '@angular/forms';

// Basic VAT number validation patterns for common countries.
const VAT_PATTERNS: { [key: string]: RegExp } = {
	'GB': /^GB\d{3}\s?\d{4}\s?\d{2}\s?\d{3}$/, // GB123 4567 89 012
	'DE': /^DE\d{9}$/, // DE123456789
	'FR': /^FR[A-Z0-9]{2}\d{9}$/, // FR12345678901
	'IT': /^IT\d{11}$/, // IT12345678901
	'ES': /^ES[A-Z0-9]\d{7}[A-Z0-9]$/, // ES12345678A
	'NL': /^NL\d{9}B\d{2}$/, // NL123456789B12
	'BE': /^BE\d{10}$/, // BE1234567890
	'AT': /^ATU\d{8}$/, // ATU12345678
	'DK': /^DK\d{8}$/, // DK12345678
	'SE': /^SE\d{12}$/, // SE123456789012
	'NO': /^NO\d{9}MVA$/, // NO123456789MVA
	'CA': /^CA\d{9}RT\d{4}$/, // CA123456789RT0001
	'AU': /^\d{11}$/, // 12345678901
	'US': /^\d{2}-\d{7}$/, // 12-3456789
	'DEFAULT': /^[A-Z0-9]{5,20}$/ // Generic pattern for other countries
};

export function vatNumberValidator(): ValidatorFn {
	return (control: AbstractControl): ValidationErrors | null => {
		if (!control.value) {
			return null;
		}

		const vatNumber = control.value.trim().toUpperCase();

		for (const [, pattern] of Object.entries(VAT_PATTERNS)) {
			if (pattern.test(vatNumber)) {
				return null; // Valid VAT number
			}
		}

		// If no specific pattern matches, use generic validation
		if (VAT_PATTERNS.DEFAULT.test(vatNumber)) {
			return null; // Acceptable format
		}

		return { invalidVatNumber: true };
	};
}
