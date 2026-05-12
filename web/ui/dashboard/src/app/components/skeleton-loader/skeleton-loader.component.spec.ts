import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SkeletonLoaderComponent } from './skeleton-loader.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SkeletonLoaderComponent', () => {
  let component: SkeletonLoaderComponent;
  let fixture: ComponentFixture<SkeletonLoaderComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SkeletonLoaderComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SkeletonLoaderComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
