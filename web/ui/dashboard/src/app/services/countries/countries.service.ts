import {Injectable} from '@angular/core';
import {HttpClient} from '@angular/common/http';
import {Observable} from 'rxjs';
import {map} from 'rxjs/operators';

export interface PopulationCount {
  year: string;
  value: string;
  sex: string;
  reliabilty: string;
}

export interface CityData {
  city: string;
  country: string;
  populationCounts: PopulationCount[];
}

export interface CountriesResponse {
  error: boolean;
  msg: string;
  data: CityData[];
}

@Injectable({
  providedIn: 'root'
})
export class CountriesService {
  private readonly API_BASE_URL = 'https://countriesnow.space/api/v0.1';

  constructor(private http: HttpClient) {}

  /**
   * Fetch all cities with population data from the API
   */
  getCitiesData(): Observable<CountriesResponse> {
    return this.http.get<CountriesResponse>(`${this.API_BASE_URL}/countries/population/cities`);
  }

  /**
   * Get all unique countries
   */
  getCountries(): Observable<{ code: string; name: string }[]> {
    return this.getCitiesData().pipe(
      map(response => {
        if (response.error || !response.data) {
          return [];
        }

        // Get unique countries and filter out invalid entries
        const countrySet = new Set<string>();
        response.data.forEach(city => {
          // Filter out invalid country names (numbers, empty strings, etc.)
          if (city.country &&
              city.country.trim() !== '' &&
              !/^\d+$/.test(city.country) && // Exclude pure numbers
              city.country.length > 1) { // Exclude single characters
            countrySet.add(city.country);
          }
        });

        return Array.from(countrySet)
          .sort()
          .map(countryName => ({
            code: this.getCountryCode(countryName),
            name: countryName
          }));
      })
    );
  }

  /**
   * Get cities for a specific country
   */
  getCitiesForCountry(countryName: string): Observable<string[]> {
    return this.getCitiesData().pipe(
      map(response => {
        if (response.error || !response.data) {
          return [];
        }

        const cities = response.data
          .filter(city => city.country === countryName)
          .map(city => city.city);

        return [...new Set(cities)].sort(); // Remove duplicates and sort
      })
    );
  }

  /**
   * Log the API response structure for analysis
   */
  analyzeApiResponse(): Observable<void> {
    return new Observable(observer => {
      this.getCitiesData().subscribe({
        next: (response) => {
          console.log('Full API Response:', response);
          console.log('Response structure:', {
            error: response.error,
            msg: response.msg,
            dataLength: response.data?.length || 0
          });

          if (response.data && response.data.length > 0) {
            console.log('Sample city data:', response.data[0]);
            console.log('Unique countries:', this.getUniqueCountries(response.data));
            console.log('Total cities:', response.data.length);
          }

          observer.next();
          observer.complete();
        },
        error: (error) => {
          console.error('API Error:', error);
          observer.error(error);
        }
      });
    });
  }

  /**
   * Get unique countries from the data
   */
  private getUniqueCountries(data: CityData[]): string[] {
    const countries = new Set<string>();
    data.forEach(city => countries.add(city.country));
    return Array.from(countries).sort();
  }

  /**
   * Get ISO country code from country name
   * This is a simplified mapping - you might want to use a more comprehensive library
   */
  private getCountryCode(countryName: string): string {
    const countryCodeMap: { [key: string]: string } = {
      'United States': 'US',
      'United States of America': 'US',
      'United Kingdom': 'GB',
      'United Kingdom of Great Britain and Northern Ireland': 'GB',
      'Canada': 'CA',
      'Australia': 'AU',
      'Germany': 'DE',
      'France': 'FR',
      'Netherlands': 'NL',
      'Sweden': 'SE',
      'Norway': 'NO',
      'Denmark': 'DK',
      'Albania': 'AL',
      'Algeria': 'DZ',
      'American Samoa': 'AS',
      'Andorra': 'AD',
      'Angola': 'AO',
      'Argentina': 'AR',
      'Austria': 'AT',
      'Belgium': 'BE',
      'Brazil': 'BR',
      'China': 'CN',
      'India': 'IN',
      'Japan': 'JP',
      'Mexico': 'MX',
      'New Zealand': 'NZ',
      'South Africa': 'ZA',
      'Spain': 'ES',
      'Switzerland': 'CH',
      'Turkey': 'TR'
    };

    return countryCodeMap[countryName] || countryName.substring(0, 2).toUpperCase();
  }
}
