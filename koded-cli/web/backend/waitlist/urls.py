from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import WaitlistViewSet, TotalUsersView

router = DefaultRouter()
router.register(r'users', WaitlistViewSet, basename='waitlist')

urlpatterns = [
    path('', include(router.urls)),
    path('total/', TotalUsersView.as_view(), name='total-users'),
]