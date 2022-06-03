import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSubscriptionComponent } from './create-subscription.component';
import { ReactiveFormsModule } from '@angular/forms';

@NgModule({
	declarations: [CreateSubscriptionComponent],
	imports: [CommonModule, ReactiveFormsModule],
	exports: [CreateSubscriptionComponent]
})
export class CreateSubscriptionModule {}
