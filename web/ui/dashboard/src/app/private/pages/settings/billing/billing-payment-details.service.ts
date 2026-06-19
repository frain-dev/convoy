import {Injectable} from '@angular/core';
import {from, Observable, of, Subject} from 'rxjs';
import {catchError, map, mergeMap} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';

export interface PaymentMethodUpdate {
  cardholderName: string;
  cardNumber: string;
  expiryMonth: string;
  expiryYear: string;
  cvv: string;
}

export interface PaymentMethodDetails {
  cardholderName: string;
  last4: string;
  brand: string;
  expiryMonth: string;
  expiryYear: string;
}

export interface PaymentMethod {
  id: string;
  card_type: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  defaulted_at: string | null;
}

export interface BillingAddressUpdate {
  name: string;
  addressLine1: string;
  addressLine2?: string;
  country: string;
  state?: string;
  city: string;
  zipCode: string;
}

export interface BillingAddressDetails {
  name: string;
  addressLine1: string;
  addressLine2?: string;
  country: string;
  state?: string;
  city: string;
  zipCode: string;
}

export interface VatInfoUpdate {
  businessName: string;
  country: string;
  vatNumber: string;
}

export interface VatInfoDetails {
  businessName: string;
  country: string;
  vatNumber: string;
}

@Injectable({ providedIn: 'root' })
export class BillingPaymentDetailsService {
  private billingStrategy: 'oss' | 'cloud' | 'licensed_self_hosted' = 'cloud';

  // Emitted when a post-checkout subscription poll confirms the new subscription
  // is active. The billing page reloads its data so the plan card, Manage plan,
  // and payment details reflect the activated subscription without a full refresh.
  private checkoutSubscriptionVerifiedSource = new Subject<void>();
  checkoutSubscriptionVerified$ = this.checkoutSubscriptionVerifiedSource.asObservable();

  constructor(private httpService: HttpService) {}

  notifyCheckoutSubscriptionVerified(): void {
    this.checkoutSubscriptionVerifiedSource.next();
  }

  setBillingStrategy(strategy: 'oss' | 'cloud' | 'licensed_self_hosted'): void {
    this.billingStrategy = strategy;
  }

  getBillingConfig(): Observable<any> {
    return from(this.httpService.request({
      url: '/billing/config',
      method: 'get'
    })).pipe(
      catchError((error) => {
        console.error('Failed to fetch billing configuration:', error);
        throw error;
      })
    );
  }


  getInternalOrganisationId(externalOrgId: string): Observable<any> {
    return from(this.httpService.request({
      url: `/billing/organisations/${externalOrgId}/internal_id`,
      method: 'get'
    })).pipe(
      catchError((error) => {
        console.error('Failed to fetch internal organisation ID:', error);
        throw error;
      })
    );
  }

  getPaymentMethods(): Observable<PaymentMethod[]> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_payment_methods'
      : `/billing/organisations/${orgId}/payment_methods`;
    return from(this.httpService.request({
      url,
      method: 'get'
    })).pipe(
      map((response: any) => {
        return response.data || [];
      }),
      catchError((error) => {
        console.error('Failed to fetch payment methods:', error);
        return of([]);
      })
    );
  }

  getPaymentMethodDetails(): Observable<PaymentMethodDetails> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_payment_methods'
      : `/billing/organisations/${orgId}/payment_methods`;
    return from(this.httpService.request({
      url,
      method: 'get'
    })).pipe(
      map((response: any) => {
        // The API returns an array, get the first/default payment method
        const paymentMethods = response.data || [];
        if (paymentMethods.length > 0) {
          const pm = paymentMethods[0];
          return {
            cardholderName: pm.cardholder_name || 'Cardholder Name',
            last4: pm.last4 || '0000',
            brand: pm.card_type || pm.brand || 'unknown',
            expiryMonth: pm.exp_month?.toString() || '',
            expiryYear: pm.exp_year?.toString() || ''
          };
        }
        return {
          cardholderName: '',
          last4: '',
          brand: '',
          expiryMonth: '',
          expiryYear: ''
        };
      }),
      catchError((error) => {
        console.error('Failed to fetch payment method details:', error);
        return of({
          cardholderName: '',
          last4: '',
          brand: '',
          expiryMonth: '',
          expiryYear: ''
        });
      })
    );
  }

  setDefaultPaymentMethod(pmId: string): Observable<any> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? `/billing/sh_payment_methods/${pmId}/default`
      : `/billing/organisations/${orgId}/payment_methods/${pmId}/default`;

    return from(this.httpService.request({
      url,
      method: 'put'
    }));
  }

  deletePaymentMethod(pmId: string): Observable<any> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? `/billing/sh_payment_methods/${pmId}`
      : `/billing/organisations/${orgId}/payment_methods/${pmId}`;

    return from(this.httpService.request({
      url,
      method: 'delete'
    }));
  }

  getBillingAddress(): Observable<BillingAddressDetails> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_organisation'
      : `/billing/organisations/${orgId}`;

    return from(this.httpService.request({
      url,
      method: 'get'
    })).pipe(
      map((response: any) => {
        const org = response.data || {};
        // Handle null/undefined values properly - use nullish coalescing to preserve empty strings
        const mapped = {
          name: org.billing_name ?? org.name ?? '',
          addressLine1: org.billing_address ?? '',
          addressLine2: org.billing_address_line2 ?? '',
          country: org.billing_country ?? '',
          state: org.billing_state ?? '',
          city: org.billing_city ?? '',
          zipCode: org.billing_zip ?? ''
        };
        return mapped;
      }),
      catchError((error) => {
        console.error('Failed to fetch billing address:', error);
        return of({
          name: '',
          addressLine1: '',
          addressLine2: '',
          country: '',
          state: '',
          city: '',
          zipCode: ''
        });
      })
    );
  }

  getVatInfo(): Observable<VatInfoDetails> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_organisation'
      : `/billing/organisations/${orgId}`;

    return from(this.httpService.request({
      url,
      method: 'get'
    })).pipe(
      map((response: any) => {
        const org = response.data || {};
        return {
          businessName: org.billing_name ?? org.name ?? 'Business Name',
          country: org.billing_country || '',
          vatNumber: org.tax_number || ''
        };
      }),
      catchError((error) => {
        console.error('Failed to fetch VAT info:', error);
        if (this.billingStrategy === 'licensed_self_hosted') {
          return of({
            businessName: '',
            country: '',
            vatNumber: ''
          });
        }

        // Fallback to organisation data if billing data not available
        return from(this.httpService.request({
          url: `/organisations/${orgId}`,
          method: 'get'
        })).pipe(
          map((response: any) => {
            const org = response.data || {};
            return {
              businessName: org.name || 'Business Name',
              country: '', // No billing data available
              vatNumber: ''
            };
          }),
          catchError(() => of({
            businessName: '',
            country: '',
            vatNumber: ''
          }))
        );
      })
    );
  }

  getSetupIntent(): Observable<any> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_payment_methods/setup_intent'
      : `/billing/organisations/${orgId}/payment_methods/setup_intent`;

    return from(this.httpService.request({
      url,
      method: 'get'
    }));
  }

  getTaxIDTypes(): Observable<any> {
    return from(this.httpService.request({
      url: '/billing/tax_id_types',
      method: 'get'
    }));
  }

  updatePaymentMethod(paymentMethod: PaymentMethodUpdate, returnFullError: boolean = false): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/billing/organisations/${orgId}/payment_methods`,
      method: 'post',
      body: paymentMethod,
      returnFullError: returnFullError
    }));
  }

  updateBillingAddress(billingAddress: BillingAddressUpdate): Observable<any> {
    const orgId = this.getOrganisationId();
    const url = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_address'
      : `/billing/organisations/${orgId}/address`;

    const addressData = {
      billing_name: billingAddress.name,
      billing_address: billingAddress.addressLine1,
      billing_address_line2: billingAddress.addressLine2 || '',
      billing_city: billingAddress.city,
      billing_state: billingAddress.state || '',
      billing_zip: billingAddress.zipCode,
      billing_country: billingAddress.country
    };

    return from(this.httpService.request({
      url,
      method: 'put',
      body: addressData
    }));
  }

  updateVatInfo(vatInfo: VatInfoUpdate): Observable<any> {
    const orgId = this.getOrganisationId();
    const orgUrl = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_organisation'
      : `/billing/organisations/${orgId}`;
    const taxUrl = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_tax_id'
      : `/billing/organisations/${orgId}/tax_id`;
    const addressUrl = this.billingStrategy === 'licensed_self_hosted'
      ? '/billing/sh_address'
      : `/billing/organisations/${orgId}/address`;

    const orgUpdateData = {
      name: vatInfo.businessName
    };
    return this.getTaxIdTypeForCountry(vatInfo.country).pipe(
      mergeMap((taxIdType: string) => {
        const taxData = {
          tax_id_type: taxIdType,
          tax_number: vatInfo.vatNumber
        };

        // Fetch current organisation data to preserve existing address fields
        return from(this.httpService.request({
          url: orgUrl,
          method: 'get'
        })).pipe(
          mergeMap((orgResponse: any) => {
            const org = orgResponse.data || {};
            // Include all existing address fields, only update the country
            const addressData = {
              billing_name: vatInfo.businessName,
              billing_address: org.billing_address || '',
              billing_address_line2: org.billing_address_line2 || '',
              billing_city: org.billing_city || '',
              billing_state: org.billing_state || '',
              billing_zip: org.billing_zip || '',
              billing_country: vatInfo.country
            };

            const orgUpdate: Observable<any> = this.billingStrategy === 'licensed_self_hosted'
              ? of(null)
              : from(this.httpService.request({
                url: `/organisations/${orgId}`,
                method: 'put',
                body: orgUpdateData
              }));

            return orgUpdate.pipe(
              mergeMap(() => {
                return from(this.httpService.request({
                  url: taxUrl,
                  method: 'put',
                  body: taxData
                }));
              }),
              mergeMap(() => {
                return from(this.httpService.request({
                  url: addressUrl,
                  method: 'put',
                  body: addressData
                }));
              })
            );
          })
        );
      })
    );
  }

  private getTaxIdTypeForCountry(countryCode: string): Observable<string> {
    return this.getTaxIDTypes().pipe(
      map((response: any) => {
        const taxIdTypes = response.data || [];

        const countryToTaxIdMap: { [key: string]: string } = {};
        taxIdTypes.forEach((taxType: any) => {
          const type = taxType.type;
          if (type) {
            const typeCountryCode = type.split('_')[0];
            if (typeCountryCode) {
              countryToTaxIdMap[typeCountryCode.toLowerCase()] = type;
            }
          }
        });

        let taxIdType = countryToTaxIdMap[countryCode.toLowerCase()];

        if (!taxIdType) {
          console.warn(`No tax ID type found for country code: ${countryCode}`);
          taxIdType = 'us_ein';
        }

        return taxIdType;
      }),
      catchError((error) => {
        console.error('Failed to fetch tax ID types:', error);
        return of('us_ein');
      })
    );
  }



  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');

    if (!org) {
      throw new Error('No organisation found. Please refresh the page and try again.');
    }

    try {
      const orgData = JSON.parse(org);

      if (!orgData.uid) {
        throw new Error('Invalid organisation data. Please refresh the page and try again.');
      }

      return orgData.uid;
    } catch (error) {
      console.error('Error parsing organisation data from localStorage:', error);
      throw new Error('Invalid organisation data. Please refresh the page and try again.');
    }
  }
}
