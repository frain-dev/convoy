import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSubscriptionComponent } from './create-subscription.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CreateAppModule } from '../create-app/create-app.module';
import { CreateSourceModule } from '../create-source/create-source.module';
import { LoaderModule } from '../loader/loader.module';
import { CreateEndpointModule } from '../../pages/project/apps/app-details/create-endpoint/create-endpoint.module';
import { InputComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ConfirmationModalComponent } from '../confirmation-modal/confirmation-modal.component';
import { FormatSecondsPipe } from 'src/app/pipes/formatSeconds/format-seconds.pipe';

@NgModule({
	declarations: [CreateSubscriptionComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		CreateAppModule,
		CreateSourceModule,
		LoaderModule,
		CreateEndpointModule,
		InputComponent,
		SelectComponent,
		ButtonComponent,
		TooltipComponent,
		ToggleComponent,
		ModalComponent,
		CardComponent,
		RadioComponent,
		ConfirmationModalComponent,
		FormatSecondsPipe,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent
	],
	exports: [CreateSubscriptionComponent]
})
export class CreateSubscriptionModule {}
