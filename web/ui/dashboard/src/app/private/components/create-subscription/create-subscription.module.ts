import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSubscriptionComponent } from './create-subscription.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CreateSourceModule } from '../create-source/create-source.module';
import { LoaderModule } from '../loader/loader.module';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { FormatSecondsPipe } from 'src/app/pipes/formatSeconds/format-seconds.pipe';
import { CreateSubscriptionFilterComponent } from '../create-subscription-filter/create-subscription-filter.component';
import { CreateEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { FormLoaderComponent } from 'src/app/components/form-loader/form-loader.component';

@NgModule({
	declarations: [CreateSubscriptionComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		CreateSourceModule,
		LoaderModule,
		CreateEndpointComponent,
		SelectComponent,
		ButtonComponent,
		TooltipComponent,
		ToggleComponent,
		ModalComponent,
		CardComponent,
		RadioComponent,
		FormatSecondsPipe,
        CreateSubscriptionFilterComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent,
        FormLoaderComponent
	],
	exports: [CreateSubscriptionComponent]
})
export class CreateSubscriptionModule {}
