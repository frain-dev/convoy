import {Component, ElementRef, EventEmitter, Input, OnDestroy, OnInit, Output, ViewChild} from '@angular/core';
import {loadStripe, Stripe, StripeCardElement, StripeCardElementChangeEvent, StripeElements} from '@stripe/stripe-js';

@Component({
  selector: 'app-stripe-elements',
  template: `
    <div class="stripe-elements-container">
      <div id="card-element" class="stripe-card-element">
        <!-- Stripe Elements will create form elements here -->
      </div>
      <div id="card-errors" class="stripe-errors" *ngIf="errorMessage">
        {{ errorMessage }}
      </div>
    </div>
  `,
  styles: [`
    .stripe-elements-container {
      margin: 16px 0;
    }

    .stripe-card-element {
      padding: 12px;
      border: 1px solid #d1d5db;
      border-radius: 6px;
      background: white;
      transition: border-color 0.15s ease-in-out;
    }

    .stripe-card-element:focus {
      border-color: #3b82f6;
      outline: none;
      box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
    }

    .stripe-errors {
      color: #dc2626;
      font-size: 14px;
      margin-top: 8px;
    }
  `]
})
export class StripeElementsComponent implements OnInit, OnDestroy {
  @Input() publishableKey: string = '';
  @Input() clientSecret: string = '';
  @Input() organisationId: string = '';
  @Output() paymentMethodCreated = new EventEmitter<string>();
  @Output() error = new EventEmitter<string>();
  @ViewChild('cardElement') cardElementRef!: ElementRef;

  private stripe: Stripe | null = null;
  private elements: StripeElements | null = null;
  private cardElement: StripeCardElement | null = null;
  errorMessage: string = '';

  async ngOnInit() {
    if (this.publishableKey && this.clientSecret) {
      await this.initializeStripe();
    }
  }

  ngOnDestroy() {
    if (this.cardElement) {
      this.cardElement.destroy();
    }
  }

  private async initializeStripe() {
    try {
      // Load Stripe with proper configuration
      this.stripe = await loadStripe(this.publishableKey);

      if (!this.stripe) {
        throw new Error('Failed to load Stripe');
      }

      this.elements = this.stripe.elements({
        clientSecret: this.clientSecret,
        appearance: {
          theme: 'stripe',
          variables: {
            colorPrimary: '#3b82f6',
            colorBackground: '#ffffff',
            colorText: '#374151',
            colorDanger: '#dc2626',
            fontFamily: '"Inter", system-ui, sans-serif',
            spacingUnit: '4px',
            borderRadius: '6px',
          }
        }
      });

      this.cardElement = this.elements.create('card', {
        style: {
          base: {
            fontSize: '16px',
            color: '#374151',
            fontFamily: '"Inter", system-ui, sans-serif',
            '::placeholder': {
              color: '#9ca3af',
            },
          },
          invalid: {
            color: '#dc2626',
          },
        },
      });

      // Mount the card element
      this.cardElement.mount('#card-element');

      // Listen for changes
      this.cardElement.on('change', (event: StripeCardElementChangeEvent) => {
        if (event.error) {
          this.errorMessage = event.error.message;
          this.error.emit(event.error.message);
        } else {
          this.errorMessage = '';
        }
      });

    } catch (err) {
      console.error('Error initializing Stripe:', err);
      this.error.emit('Failed to initialize payment form');
    }
  }

  async confirmSetup(): Promise<boolean> {
    if (!this.stripe || !this.elements || !this.clientSecret) {
      this.error.emit('Payment form not initialized');
      return false;
    }

    try {
      const { error } = await this.stripe.confirmCardSetup(this.clientSecret, {
        payment_method: {
          card: this.cardElement!,
          metadata: {
            id: this.organisationId
          }
        }
      });

      if (error) {
        this.errorMessage = error.message || 'Payment failed';
        this.error.emit(error.message || 'Payment failed');
        return false;
      } else {
        this.paymentMethodCreated.emit('success');
        return true;
      }
    } catch (err) {
      console.error('Error confirming setup:', err);
      this.error.emit('Payment confirmation failed');
      return false;
    }
  }

  // Method to update client secret when it changes
  async updateClientSecret(newClientSecret: string) {
    console.log('Updating client secret:', newClientSecret ? 'Present' : 'Missing');
    this.clientSecret = newClientSecret;

    // Destroy existing elements
    if (this.cardElement) {
      this.cardElement.destroy();
      this.cardElement = null;
    }

    // Reinitialize with new client secret
    if (this.stripe && this.clientSecret) {
      await this.initializeStripe();
    }
  }

  // Public method to trigger payment method confirmation
  async confirmPaymentMethod(): Promise<boolean> {
    console.log('StripeElementsComponent: confirmPaymentMethod called');
    return await this.confirmSetup();
  }
}
