import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateAppComponent } from './create-app.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { LoaderModule } from '../loader/loader.module';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@NgModule({
	declarations: [CreateAppComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, LoaderModule, InputComponent, ButtonComponent],
	exports: [CreateAppComponent]
})
export class CreateAppModule {}
