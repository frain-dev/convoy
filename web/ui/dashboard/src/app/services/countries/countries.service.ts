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

interface CountriesIsoResponse {
  error: boolean;
  msg: string;
  data: { name: string; Iso2: string; Iso3: string }[];
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
   * Get all unique countries with ISO codes
   */
  getCountries(): Observable<{ code: string; name: string }[]> {
    return this.http.get<CountriesIsoResponse>(`${this.API_BASE_URL}/countries/iso`).pipe(
      map(response => {
        if (response.error || !response.data) {
          return [];
        }

        return response.data
          .filter(c => !!c.name && !!c.Iso2)
          .map(c => ({ code: c.Iso2, name: c.name }))
          .sort((a, b) => a.name.localeCompare(b.name));
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
}
