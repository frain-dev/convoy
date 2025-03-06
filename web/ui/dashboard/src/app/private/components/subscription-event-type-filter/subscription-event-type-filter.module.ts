import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { SubscriptionEventTypeFilterComponent } from './subscription-event-type-filter.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { CreateSubscriptionFilterComponent } from '../create-subscription-filter/create-subscription-filter.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';

@NgModule({
	declarations: [SubscriptionEventTypeFilterComponent],
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, SelectComponent, CreateSubscriptionFilterComponent, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	exports: [SubscriptionEventTypeFilterComponent]
})
export class SubscriptionEventTypeFilterModule {}
