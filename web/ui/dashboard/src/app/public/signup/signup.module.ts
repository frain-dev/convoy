import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SignupComponent } from './signup.component';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent } from 'src/app/components/input/input.component';

const routes: Routes = [{ path: '', component: SignupComponent }];

@NgModule({
	declarations: [SignupComponent],
	imports: [CommonModule, ReactiveFormsModule, RouterModule.forChild(routes), ButtonComponent, InputComponent]
})
export class SignupModule {}
