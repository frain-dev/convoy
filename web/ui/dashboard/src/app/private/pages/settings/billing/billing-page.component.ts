import {Component, ElementRef, OnInit, ViewChild} from '@angular/core';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {
    BillingAddressDetails,
    BillingPaymentDetailsService,
    PaymentMethodDetails,
    VatInfoDetails
} from './billing-payment-details.service';
import {CardIconService} from './card-icon.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {CountriesService} from 'src/app/services/countries/countries.service';

@Component({
  selector: 'app-billing-page',
  templateUrl: './billing-page.component.html',
  styleUrls: ['./billing-page.component.scss']
})
export class BillingPageComponent implements OnInit {
  @ViewChild('paymentDetailsDialog') paymentDetailsDialog!: ElementRef<HTMLDialogElement>;

  isPaymentDetailsOpen = false;
  refreshOverviewTrigger = 0;
  currentYear = new Date().getFullYear() - 2000; // 2-digit current year

  // Existing data
  paymentMethodDetails: PaymentMethodDetails | null = null;
  billingAddressDetails: BillingAddressDetails | null = null;
  vatInfoDetails: VatInfoDetails | null = null;

  // Edit states
  isEditingPaymentMethod = false;
  isEditingBillingAddress = false;
  isEditingVat = false;

  // Loading states
  isLoadingPaymentMethod = false;
  isLoadingBillingAddress = false;
  isLoadingVat = false;

  paymentMethodForm!: FormGroup;
  billingAddressForm!: FormGroup;
  vatForm!: FormGroup;

  countries: { code: string; name: string }[] = [];
  cities: string[] = [];
  isLoadingCountries = false;
  isLoadingCities = false;

  // API error message
  apiError = '';

  constructor(
    private fb: FormBuilder,
    private billingPaymentDetailsService: BillingPaymentDetailsService,
    private generalService: GeneralService,
    private cardIconService: CardIconService,
    private countriesService: CountriesService
  ) {
    this.initializeForms();
  }

  ngOnInit() {
    this.loadCountries();

    // Subscribe to country changes in the form
    this.billingAddressForm.get('country')?.valueChanges.subscribe(countryName => {
      this.onCountryChange(countryName);
    });
  }

  private loadCountries() {
    this.isLoadingCountries = true;
    this.countriesService.getCountries().subscribe({
      next: (countries) => {
        this.countries = countries;
        this.isLoadingCountries = false;
        console.log('Loaded countries:', countries.length);
      },
      error: (error) => {
        console.error('Failed to load countries:', error);
        this.isLoadingCountries = false;
        // No fallback data - countries array remains empty if API fails
        this.countries = [];
      }
    });
  }

  onCountryChange(countryName: string) {
    if (!countryName) {
      this.cities = [];
      return;
    }

    this.isLoadingCities = true;
    this.countriesService.getCitiesForCountry(countryName).subscribe({
      next: (cities) => {
        this.cities = cities;
        this.isLoadingCities = false;
        console.log(`Loaded ${cities.length} cities for ${countryName}`);
      },
      error: (error) => {
        console.error('Failed to load cities:', error);
        this.isLoadingCities = false;
        this.cities = [];
      }
    });
  }

  private initializeForms() {
    this.paymentMethodForm = this.fb.group({
      cardholderName: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(100)]],
      cardNumber: ['', [Validators.required, this.cardNumberValidator()]],
      expiryMonth: ['', [Validators.required, Validators.min(1), Validators.max(12)]],
      expiryYear: ['', [Validators.required, Validators.min(this.currentYear)]],
      cvv: ['', [Validators.required, Validators.pattern(/^\d{3,4}$/)]]
    });

    this.billingAddressForm = this.fb.group({
      name: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(100)]],
      addressLine1: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(200)]],
      addressLine2: ['', [Validators.maxLength(200)]],
      country: ['', Validators.required],
      city: ['', Validators.required],
      zipCode: ['', [Validators.required, Validators.minLength(3), Validators.maxLength(20), this.zipCodeValidator()]]
    });

    this.vatForm = this.fb.group({
      businessName: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(200)]],
      country: ['', Validators.required],
      vatNumber: ['', [Validators.required, this.vatNumberValidator()]]
    });
  }

  openPaymentDetails() {
    this.paymentDetailsDialog.nativeElement.showModal();
    this.loadExistingData();
  }

  closePaymentDetails() {
    this.paymentDetailsDialog.nativeElement.close();
    this.resetEditStates();
    this.resetForms();
  }

  private loadExistingData() {
    this.loadPaymentMethodDetails();
    this.loadBillingAddress();
    this.loadVatInfo();
  }

  private loadPaymentMethodDetails() {
    this.isLoadingPaymentMethod = true;
    this.billingPaymentDetailsService.getPaymentMethodDetails().subscribe({
      next: (details) => {
        this.paymentMethodDetails = details;
        this.isLoadingPaymentMethod = false;
      },
      error: (error) => {
        console.error('Failed to load payment method details:', error);
        this.isLoadingPaymentMethod = false;
      }
    });
  }

  private loadBillingAddress() {
    this.isLoadingBillingAddress = true;
    this.billingPaymentDetailsService.getBillingAddress().subscribe({
      next: (details) => {
        this.billingAddressDetails = details;
        this.isLoadingBillingAddress = false;
      },
      error: (error) => {
        console.error('Failed to load billing address:', error);
        this.isLoadingBillingAddress = false;
      }
    });
  }

  private loadVatInfo() {
    this.isLoadingVat = true;
    this.billingPaymentDetailsService.getVatInfo().subscribe({
      next: (details) => {
        this.vatInfoDetails = details;
        this.isLoadingVat = false;
      },
      error: (error) => {
        console.error('Failed to load VAT info:', error);
        this.isLoadingVat = false;
      }
    });
  }

  // Edit mode methods
  startEditingPaymentMethod() {
    this.isEditingPaymentMethod = true;
    // Don't prefill sensitive payment information for security
    this.paymentMethodForm.reset();
  }

  startEditingBillingAddress() {
    this.isEditingBillingAddress = true;
    if (this.billingAddressDetails) {
      this.billingAddressForm.patchValue(this.billingAddressDetails);
    }
  }

  startEditingVat() {
    this.isEditingVat = true;
    if (this.vatInfoDetails) {
      this.vatForm.patchValue(this.vatInfoDetails);
    }
  }

  cancelEditingPaymentMethod() {
    this.isEditingPaymentMethod = false;
    this.paymentMethodForm.reset();
  }

  cancelEditingBillingAddress() {
    this.isEditingBillingAddress = false;
    this.billingAddressForm.reset();
  }

  cancelEditingVat() {
    this.isEditingVat = false;
    this.vatForm.reset();
  }

  private resetEditStates() {
    this.isEditingPaymentMethod = false;
    this.isEditingBillingAddress = false;
    this.isEditingVat = false;
  }

  private resetForms() {
    this.paymentMethodForm.reset();
    this.billingAddressForm.reset();
    this.vatForm.reset();
    this.apiError = '';
  }

  onUpdatePaymentMethod() {
    console.log('Payment method form valid:', this.paymentMethodForm.valid);
    console.log('Payment method form errors:', this.paymentMethodForm.errors);
    console.log('Payment method form value:', this.paymentMethodForm.value);

    if (this.paymentMethodForm.valid) {
      console.log('Updating payment method:', this.paymentMethodForm.value);

      // Clean the card number and ensure expiry fields are strings
      const formData = { ...this.paymentMethodForm.value };
      formData.cardNumber = formData.cardNumber.replaceAll(' ', '');

      // Convert expiry month and year to strings (mock service expects strings)
      if (typeof formData.expiryMonth === 'number') {
        formData.expiryMonth = formData.expiryMonth.toString();
      }
      if (typeof formData.expiryYear === 'number') {
        formData.expiryYear = formData.expiryYear.toString();
      }

      console.log('Processed form data:', formData);

      this.billingPaymentDetailsService.updatePaymentMethod(formData, true).subscribe({
        next: (response) => {
          console.log('Payment method updated successfully:', response);
          this.generalService.showNotification({
            message: 'Payment method updated successfully!',
            style: 'success'
          });
          this.isEditingPaymentMethod = false;
          this.loadPaymentMethodDetails(); // Refresh the data
          this.refreshOverviewTrigger++;
        },
        error: (error) => {
          console.error('Failed to update payment method:', error);

          // Extract specific error message from the response
          let errorMessage = 'Failed to update payment method. Please try again.';

          if (error.response?.data?.message) {
            const fullMessage = error.response.data.message;
            // Extract the part after "billing service error: " if it exists
            if (fullMessage.includes('billing service error: ')) {
              errorMessage = fullMessage.split('billing service error: ')[1];
            } else {
              errorMessage = fullMessage;
            }
          } else if (error.error && error.error.message) {
            errorMessage = error.error.message;
          } else if (error.message) {
            errorMessage = error.message;
          } else if (error.status === 400) {
            errorMessage = 'Invalid request data. Please check your card details.';
          } else if (error.status === 500) {
            errorMessage = 'Server error. Please try again later.';
          }

          this.apiError = errorMessage;
        }
      });
    } else {
      console.log('Form is invalid. Marking fields as touched...');
      this.markFormGroupTouched(this.paymentMethodForm);
    }
  }

  onUpdateBillingAddress() {
    if (this.billingAddressForm.valid) {
      console.log('Updating billing address:', this.billingAddressForm.value);

      this.billingPaymentDetailsService.updateBillingAddress(this.billingAddressForm.value).subscribe({
        next: (response) => {
          console.log('Billing address updated successfully:', response);
          this.generalService.showNotification({
            message: 'Billing address updated successfully!',
            style: 'success'
          });
          this.isEditingBillingAddress = false;
          this.loadBillingAddress(); // Refresh the data
        },
        error: (error) => {
          console.error('Failed to update billing address:', error);
          this.generalService.showNotification({
            message: 'Failed to update billing address. Please try again.',
            style: 'error'
          });
        }
      });
    } else {
      this.markFormGroupTouched(this.billingAddressForm);
    }
  }

  onUpdateVatInfo() {
    if (this.vatForm.valid) {
      console.log('Updating VAT info:', this.vatForm.value);

      this.billingPaymentDetailsService.updateVatInfo(this.vatForm.value).subscribe({
        next: (response) => {
          console.log('VAT information updated successfully:', response);
          this.generalService.showNotification({
            message: 'VAT information updated successfully!',
            style: 'success'
          });
          this.isEditingVat = false;
          this.loadVatInfo(); // Refresh the data
        },
        error: (error) => {
          console.error('Failed to update VAT information:', error);
          this.generalService.showNotification({
            message: 'Failed to update VAT information. Please try again.',
            style: 'error'
          });
        }
      });
    } else {
      this.markFormGroupTouched(this.vatForm);
    }
  }

  getCountryName(countryCode: string): string {
    const country = this.countries.find(c => c.code === countryCode);
    return country ? country.name : countryCode;
  }

  private markFormGroupTouched(formGroup: FormGroup) {
    Object.keys(formGroup.controls).forEach(key => {
      const control = formGroup.get(key);
      control?.markAsTouched();
    });
  }

  formatCardNumber(event: any) {
    const input = event.target;
    // Remove all non-digits and limit to 16 digits
    const cleanValue = input.value.replace(/\D/g, '').substring(0, 16);
    const formattedValue = this.formatNumber(cleanValue);
    input.value = formattedValue;

    // Update the form control and trigger validation
    this.paymentMethodForm.patchValue({ cardNumber: formattedValue });
    this.paymentMethodForm.get('cardNumber')?.updateValueAndValidity();

    // Debug logging
    console.log('Card number value:', formattedValue);
    console.log('Clean value:', cleanValue);
    console.log('Form valid:', this.paymentMethodForm.valid);
    console.log('Card number valid:', this.paymentMethodForm.get('cardNumber')?.valid);
    console.log('Card number errors:', this.paymentMethodForm.get('cardNumber')?.errors);
  }

  private formatNumber(number: string): string {
    return number.split('').reduce((seed, next, index) => {
      if (index !== 0 && !(index % 4)) seed += ' ';
      return seed + next;
    }, '');
  }

  private cardNumberValidator() {
    return (control: any) => {
      if (!control.value) {
        return null;
      }
      const cleanValue = control.value.replace(/\s/g, '');
      if (cleanValue.length === 16 && /^\d+$/.test(cleanValue)) {
        return null;
      }
      return { invalidCardNumber: true };
    };
  }

  private vatNumberValidator() {
    return (control: any) => {
      if (!control.value) {
        return null;
      }

      const vatNumber = control.value.trim().toUpperCase();

      // Basic VAT number validation patterns for common countries
      const vatPatterns: { [key: string]: RegExp } = {
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

      // Check if it matches any known pattern
      for (const [country, pattern] of Object.entries(vatPatterns)) {
        if (pattern.test(vatNumber)) {
          return null; // Valid VAT number
        }
      }

      // If no specific pattern matches, use generic validation
      if (vatPatterns.DEFAULT.test(vatNumber)) {
        return null; // Acceptable format
      }

      return { invalidVatNumber: true };
    };
  }

  private zipCodeValidator() {
    return (control: any) => {
      if (!control.value) {
        return null;
      }

      const zipCode = control.value.trim();

      // Basic zip code validation patterns for common countries
      const zipPatterns: { [key: string]: RegExp } = {
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

      // Check if it matches any known pattern
      for (const [country, pattern] of Object.entries(zipPatterns)) {
        if (pattern.test(zipCode)) {
          return null; // Valid zip code
        }
      }

      // If no specific pattern matches, use generic validation
      if (zipPatterns.DEFAULT.test(zipCode)) {
        return null; // Acceptable format
      }

      return { invalidZipCode: true };
    };
  }

  getCardIconSvg() {
    return this.cardIconService.getCardIconSvg(this.paymentMethodDetails?.brand);
  }
}
