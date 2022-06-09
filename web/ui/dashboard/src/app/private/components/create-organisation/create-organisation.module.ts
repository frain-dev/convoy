import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateOrganisationComponent } from './create-organisation.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';

@NgModule({
	declarations: [CreateOrganisationComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule],
	exports: [CreateOrganisationComponent]
})
export class CreateOrganisationModule {}
