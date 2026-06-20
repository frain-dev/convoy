import {Injectable} from '@angular/core';

export interface Country {
	code: string;
	name: string;
}

@Injectable({ providedIn: 'root' })
export class BillingLocationService {
	// Resolve a display country name, preferring the VAT country list (which is a
	// subset with tax-id types) before the full country list, falling back to the
	// raw code when neither matches.
	getCountryName(countryCode: string, vatCountries: Country[], countries: Country[]): string {
		let country = vatCountries.find(c => c.code === countryCode);
		if (!country) {
			country = countries.find(c => c.code === countryCode);
		}
		return country ? country.name : countryCode;
	}

	// Keep a previously-saved city visible even when the provider list omits it,
	// so rehydrating an existing address does not silently drop the value.
	withPreferredCity(cities: string[], preferredCity: string): string[] {
		if (!preferredCity) {
			return cities;
		}

		const match = cities.some(city => city.trim().toLowerCase() === preferredCity.trim().toLowerCase());
		if (match) {
			return cities;
		}

		return [preferredCity, ...cities];
	}

	findMatchingCity(cities: string[], preferredCity: string): string {
		if (!preferredCity) {
			return '';
		}

		return cities.find(city => city.trim().toLowerCase() === preferredCity.trim().toLowerCase()) || '';
	}
}
