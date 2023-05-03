import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSourceComponent } from './create-source.component';
import { ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { FileInputComponent } from 'src/app/components/file-input/file-input.component';
import { FormLoaderComponent } from 'src/app/components/form-loader/form-loader.component';
import { TokenModalComponent } from '../token-modal/token-modal.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { PermissionDirective } from '../permission/permission.directive';

@NgModule({
	declarations: [CreateSourceComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		ButtonComponent,
		SelectComponent,
		RadioComponent,
		CardComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent,
		FileInputComponent,
		FormLoaderComponent,
		ModalComponent,
		TokenModalComponent,
		PermissionDirective
	],
	exports: [CreateSourceComponent]
})
export class CreateSourceModule {}
