import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSourceComponent } from './create-source.component';
import { ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { CardComponent } from 'src/stories/card/card.component';

@NgModule({
	declarations: [CreateSourceComponent],
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputComponent, SelectComponent, RadioComponent, CardComponent],
	exports: [CreateSourceComponent]
})
export class CreateSourceModule {}
