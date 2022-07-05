import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateOrganisationComponent } from './create-organisation.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@NgModule({
	declarations: [CreateOrganisationComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, InputComponent, ButtonComponent],
	exports: [CreateOrganisationComponent]
})
export class CreateOrganisationModule {}
