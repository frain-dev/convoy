import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateAppComponent } from './create-app.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { LoaderModule } from '../loader/loader.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { CardComponent } from 'src/app/components/card/card.component';

@NgModule({
	declarations: [CreateAppComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, ButtonComponent, InputComponent, LoaderModule, CardComponent],
	exports: [CreateAppComponent]
})
export class CreateAppModule {}
