import { Component, ViewChild, ElementRef } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { BillingPaymentDetailsService } from './billing-payment-details.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
  selector: 'app-billing-page',
  templateUrl: './billing-page.component.html'
})
export class BillingPageComponent {
  @ViewChild('paymentDetailsDialog') paymentDetailsDialog!: ElementRef<HTMLDialogElement>;
  
  isPaymentDetailsOpen = false;
  refreshOverviewTrigger = 0;
  currentYear = new Date().getFullYear() - 2000; // 2-digit current year
  
  paymentMethodForm!: FormGroup;
  billingAddressForm!: FormGroup;
  vatForm!: FormGroup;

  countries = [
    { code: 'US', name: 'United States' },
    { code: 'GB', name: 'United Kingdom' },
    { code: 'CA', name: 'Canada' },
    { code: 'AU', name: 'Australia' },
    { code: 'DE', name: 'Germany' },
    { code: 'FR', name: 'France' },
    { code: 'NL', name: 'Netherlands' },
    { code: 'SE', name: 'Sweden' },
    { code: 'NO', name: 'Norway' },
    { code: 'DK', name: 'Denmark' }
  ];

  cities = [
    'New York', 'London', 'Toronto', 'Sydney', 'Berlin', 'Paris', 'Amsterdam', 'Stockholm', 'Oslo', 'Copenhagen'
  ];

  // API error message
  apiError = '';

  constructor(
    private fb: FormBuilder,
    private billingPaymentDetailsService: BillingPaymentDetailsService,
    private generalService: GeneralService
  ) {
    this.initializeForms();
  }

  private initializeForms() {
    this.paymentMethodForm = this.fb.group({
      cardholderName: ['John Doe', Validators.required],
      cardNumber: ['0000000000000000', [Validators.required, this.cardNumberValidator()]],
      expiryMonth: ['12', [Validators.required, Validators.min(1), Validators.max(12)]],
      expiryYear: ['26', [Validators.required, Validators.min(this.currentYear)]], // Current year as 2-digit
      cvv: ['000', [Validators.required, Validators.pattern(/^\d{3,4}$/)]]
    });

    this.billingAddressForm = this.fb.group({
      name: ['', Validators.required],
      addressLine1: ['', Validators.required],
      addressLine2: [''],
      city: ['', Validators.required],
      zipCode: ['', Validators.required]
    });

    this.vatForm = this.fb.group({
      businessName: ['', Validators.required],
      country: ['', Validators.required],
      vatNumber: ['', Validators.required]
    });
  }

  openPaymentDetails() {
    this.paymentDetailsDialog.nativeElement.showModal();
  }

  closePaymentDetails() {
    this.paymentDetailsDialog.nativeElement.close();
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
          this.closePaymentDetails();
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
          this.closePaymentDetails();
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
          this.closePaymentDetails();
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
} 