import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateAppComponent } from './create-app.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { LoaderModule } from '../loader/loader.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, PasswordInputFieldComponent } from 'src/app/components/input/input.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ConfirmationModalComponent } from '../confirmation-modal/confirmation-modal.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';

@NgModule({
	declarations: [CreateAppComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, ButtonComponent, RadioComponent, LoaderModule, CardComponent, ConfirmationModalComponent, TooltipComponent, InputFieldDirective, InputDirective, LabelComponent, InputErrorComponent, PasswordInputFieldComponent],
	exports: [CreateAppComponent]
})
export class CreateAppModule {}
